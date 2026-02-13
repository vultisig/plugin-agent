package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rlp"
)

// ProposalResponse matches the API response from /propose endpoint
type ProposalResponse struct {
	PolicyID  string          `json:"policy_id"`
	Network   string          `json:"network"`
	TxHex     string          `json:"tx_hex"`
	Signature KeysignResponse `json:"signature"`
}

// KeysignResponse from mobile-tss-lib
type KeysignResponse struct {
	R          string          `json:"r"`
	S          string          `json:"s"`
	RecoveryID string          `json:"recovery_id"`
	RBytes     json.RawMessage `json:"r_bytes,omitempty"`
	SBytes     json.RawMessage `json:"s_bytes,omitempty"`
	DerSign    json.RawMessage `json:"der_signature,omitempty"`
}

type IntegrationTest struct {
	agentURL    string
	ethRPCURL   string
	policyID    string
	fromAddress common.Address
	toAddress   common.Address
	ethClient   *ethclient.Client
	httpClient  *http.Client
}

func NewIntegrationTest(agentURL, ethRPCURL, policyID string, from, to common.Address) (*IntegrationTest, error) {
	client, err := ethclient.Dial(ethRPCURL)
	if err != nil {
		return nil, fmt.Errorf("dial eth rpc: %w", err)
	}

	return &IntegrationTest{
		agentURL:    agentURL,
		ethRPCURL:   ethRPCURL,
		policyID:    policyID,
		fromAddress: from,
		toAddress:   to,
		ethClient:   client,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (it *IntegrationTest) Close() {
	if it.ethClient != nil {
		it.ethClient.Close()
	}
}

// Step 1: Generate unsigned EIP-1559 transaction
func (it *IntegrationTest) GenerateUnsignedTx(ctx context.Context, value *big.Int) (string, error) {
	nonce, err := it.ethClient.PendingNonceAt(ctx, it.fromAddress)
	if err != nil {
		return "", fmt.Errorf("get nonce: %w", err)
	}

	chainID, err := it.ethClient.ChainID(ctx)
	if err != nil {
		return "", fmt.Errorf("get chain id: %w", err)
	}

	maxPriorityFeePerGas := new(big.Int).SetUint64(100_000_000)
	maxFeePerGas := new(big.Int).SetUint64(500_000_000)
	gasLimit := uint64(21_000)
	data := []byte{}

	unsignedPayload := []interface{}{
		uint(chainID.Uint64()),
		uint64(nonce),
		maxPriorityFeePerGas,
		maxFeePerGas,
		uint64(gasLimit),
		&it.toAddress,
		value,
		data,
		types.AccessList{},
	}

	payload, err := rlp.EncodeToBytes(unsignedPayload)
	if err != nil {
		return "", fmt.Errorf("rlp encode payload: %w", err)
	}

	txRaw := append([]byte{types.DynamicFeeTxType}, payload...)
	txHex := hex.EncodeToString(txRaw)

	log.Printf("[Step 1] Generated unsigned tx: %s", txHex)
	log.Printf("         From: %s", it.fromAddress.Hex())
	log.Printf("         To: %s", it.toAddress.Hex())
	log.Printf("         Value: %s wei", value.String())
	log.Printf("         Nonce: %d", nonce)
	log.Printf("         Chain ID: %d", chainID.Uint64())

	return txHex, nil
}

// Step 2: Propose transaction to agent server
func (it *IntegrationTest) ProposeTransaction(ctx context.Context, txHex, network string) (*ProposalResponse, error) {
	params := url.Values{}
	params.Add("policy_id", it.policyID)
	params.Add("network", network)
	params.Add("tx_hex", txHex)

	proposeURL := fmt.Sprintf("%s/propose?%s", it.agentURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, proposeURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	log.Printf("[Step 2] Proposing to: %s", proposeURL)

	resp, err := it.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var proposalResp ProposalResponse
	err = json.Unmarshal(body, &proposalResp)
	if err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	log.Printf("[Step 2] Received signature:")
	log.Printf("         R: %s", proposalResp.Signature.R)
	log.Printf("         S: %s", proposalResp.Signature.S)
	log.Printf("         RecoveryID: %s", proposalResp.Signature.RecoveryID)

	return &proposalResp, nil
}

// Step 3: Broadcast signed transaction to Ethereum mainnet
func (it *IntegrationTest) BroadcastTransaction(ctx context.Context, txHex string, sig KeysignResponse) (string, error) {
	rawTx, err := hex.DecodeString(txHex)
	if err != nil {
		return "", fmt.Errorf("decode tx hex: %w", err)
	}

	if rawTx[0] != types.DynamicFeeTxType {
		return "", fmt.Errorf("expected type 0x02, got 0x%x", rawTx[0])
	}

	payload := rawTx[1:]
	var unsigned struct {
		ChainID    uint64
		Nonce      uint64
		GasTipCap  *big.Int
		GasFeeCap  *big.Int
		Gas        uint64
		To         *common.Address `rlp:"nil"`
		Value      *big.Int
		Data       []byte
		AccessList types.AccessList
	}

	err = rlp.DecodeBytes(payload, &unsigned)
	if err != nil {
		return "", fmt.Errorf("decode unsigned payload: %w", err)
	}

	r, ok := new(big.Int).SetString(sig.R, 16)
	if !ok {
		return "", fmt.Errorf("invalid r: %s", sig.R)
	}

	s, ok := new(big.Int).SetString(sig.S, 16)
	if !ok {
		return "", fmt.Errorf("invalid s: %s", sig.S)
	}

	recoveryID := byte(0)
	if sig.RecoveryID == "1" {
		recoveryID = 1
	}
	v := big.NewInt(int64(recoveryID))

	signedTxData := &types.DynamicFeeTx{
		ChainID:    new(big.Int).SetUint64(unsigned.ChainID),
		Nonce:      unsigned.Nonce,
		GasTipCap:  unsigned.GasTipCap,
		GasFeeCap:  unsigned.GasFeeCap,
		Gas:        unsigned.Gas,
		To:         unsigned.To,
		Value:      unsigned.Value,
		Data:       unsigned.Data,
		AccessList: unsigned.AccessList,
		V:          v,
		R:          r,
		S:          s,
	}

	signedTx := types.NewTx(signedTxData)

	log.Printf("[Step 3] Broadcasting transaction...")
	log.Printf("         Tx Hash: %s", signedTx.Hash().Hex())

	err = it.ethClient.SendTransaction(ctx, signedTx)
	if err != nil {
		return "", fmt.Errorf("broadcast tx: %w", err)
	}

	txHash := signedTx.Hash().Hex()
	log.Printf("[Step 3] Transaction broadcasted successfully!")
	log.Printf("         Tx Hash: %s", txHash)
	log.Printf("         Explorer: https://etherscan.io/tx/%s", txHash)

	return txHash, nil
}

// Run executes the complete integration test
func (it *IntegrationTest) Run(ctx context.Context, value *big.Int, network string) error {
	log.Println("=== Starting Integration Test ===")
	log.Println()

	txHex, err := it.GenerateUnsignedTx(ctx, value)
	if err != nil {
		return fmt.Errorf("step 1 failed: %w", err)
	}
	log.Println()

	proposalResp, err := it.ProposeTransaction(ctx, txHex, network)
	if err != nil {
		return fmt.Errorf("step 2 failed: %w", err)
	}
	log.Println()

	txHash, err := it.BroadcastTransaction(ctx, proposalResp.TxHex, proposalResp.Signature)
	if err != nil {
		return fmt.Errorf("step 3 failed: %w", err)
	}

	log.Println()
	log.Println("=== Integration Test Completed Successfully ===")
	log.Printf("Final Tx Hash: %s", txHash)

	return nil
}

func main() {
	agentURL := os.Getenv("AGENT_URL")
	if agentURL == "" {
		log.Fatal("AGENT_URL environment variable is required")
	}

	ethRPCURL := os.Getenv("ETH_RPC_URL")
	if ethRPCURL == "" {
		log.Fatal("ETH_RPC_URL environment variable is required")
	}

	policyID := os.Getenv("POLICY_ID")
	if policyID == "" {
		log.Fatal("POLICY_ID environment variable is required")
	}

	fromAddr := os.Getenv("FROM_ADDRESS")
	if fromAddr == "" {
		log.Fatal("FROM_ADDRESS environment variable is required")
	}

	toAddr := os.Getenv("TO_ADDRESS")
	if toAddr == "" {
		log.Fatal("TO_ADDRESS environment variable is required")
	}

	valueStr := os.Getenv("VALUE_WEI")
	if valueStr == "" {
		valueStr = "1000"
	}

	network := os.Getenv("NETWORK")
	if network == "" {
		network = "Ethereum"
	}

	value, ok := new(big.Int).SetString(valueStr, 10)
	if !ok {
		log.Fatalf("Invalid VALUE_WEI: %s", valueStr)
	}

	log.Println("Configuration:")
	log.Printf("  Agent URL: %s", agentURL)
	log.Printf("  Ethereum RPC: %s", ethRPCURL)
	log.Printf("  Policy ID: %s", policyID)
	log.Printf("  From Address: %s", fromAddr)
	log.Printf("  To Address: %s", toAddr)
	log.Printf("  Value: %s wei", value.String())
	log.Printf("  Network: %s", network)
	log.Println()

	test, err := NewIntegrationTest(
		agentURL,
		ethRPCURL,
		policyID,
		common.HexToAddress(fromAddr),
		common.HexToAddress(toAddr),
	)
	if err != nil {
		log.Fatalf("Initialize test: %v", err)
	}
	defer test.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	err = test.Run(ctx, value, network)
	if err != nil {
		log.Fatalf("Test failed: %v", err)
	}
}
