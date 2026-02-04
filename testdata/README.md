# Integration Test

This directory contains integration tests for the Vultisig Plugin Agent.

## Files

- `integration.go` - Complete end-to-end integration test: generate tx → propose → broadcast

## Integration Test Flow

The integration test performs three steps:

1. **Generate Transaction**: Creates an unsigned EIP-1559 Ethereum transaction
2. **Propose to Agent**: Submits the transaction to the agent server at `/propose` endpoint
3. **Broadcast**: Broadcasts the signed transaction to Ethereum mainnet

## Prerequisites

1. Agent server running on `localhost:8081` (or set `AGENT_URL`)
2. Valid `POLICY_ID` configured in the agent database
3. Ethereum RPC access (e.g., `https://ethereum-rpc.publicnode.com`)
4. `FROM_ADDRESS` must have sufficient ETH for gas fees
5. Go 1.25+ installed

## Running the Test

Set the required environment variables and run:

```bash
export AGENT_URL="http://localhost:8081"
export ETH_RPC_URL="https://ethereum-rpc.publicnode.com"
export POLICY_ID="your-policy-uuid"
export FROM_ADDRESS="0xYourAddress"
export TO_ADDRESS="0xRecipientAddress"
export VALUE_WEI="1000"
export NETWORK="Ethereum"

cd testdata
go run integration.go
```

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `AGENT_URL` | Yes | - | Plugin agent server URL |
| `ETH_RPC_URL` | Yes | - | Ethereum RPC endpoint |
| `POLICY_ID` | Yes | - | UUID of the plugin policy |
| `FROM_ADDRESS` | Yes | - | Source Ethereum address |
| `TO_ADDRESS` | Yes | - | Destination Ethereum address |
| `VALUE_WEI` | No | `1000` | Amount to send in wei |
| `NETWORK` | No | `Ethereum` | Network identifier (Ethereum, BSC, etc.) |

## Expected Output

```
Configuration:
  Agent URL: http://localhost:8081
  Ethereum RPC: https://ethereum-rpc.publicnode.com
  Policy ID: 12345678-1234-1234-1234-123456789abc
  From Address: 0x...
  To Address: 0x...
  Value: 1000 wei
  Network: Ethereum

=== Starting Integration Test ===

[Step 1] Generated unsigned tx: 02ea0114843b9aca008504a817c80082520894e909df59d662de1c0e1579d2c1d40f0776e866ca8203e880c0
         From: 0x...
         To: 0x...
         Value: 1000 wei
         Nonce: 20
         Chain ID: 1

[Step 2] Proposing to: http://localhost:8081/propose?network=Ethereum&policy_id=...&tx_hex=...
         Received signature:
         R: 456d92c1184a652a16f80ae3b371cad591a82855b0eac7fbfd481a5259cd8901
         S: 0c15db4ff27930a27476e322d44d033dfbb7ab18abe5479bd81f90abd7100812
         RecoveryID: 1

[Step 3] Broadcasting transaction...
         Tx Hash: 0x1234567890abcdef...
[Step 3] Transaction broadcasted successfully!
         Tx Hash: 0x1234567890abcdef...
         Explorer: https://etherscan.io/tx/0x1234567890abcdef...

=== Integration Test Completed Successfully ===
Final Tx Hash: 0x1234567890abcdef...
```

## Troubleshooting

### "failed to get plugin policy"

- Ensure the `POLICY_ID` is valid and exists in the agent database
- Check that the agent server is running and accessible

### "failed to sign request"

- Verify the vault associated with the policy is properly configured
- Check agent server logs for detailed error messages
- Ensure the worker is running and processing tasks

### "insufficient funds for gas"

- The `FROM_ADDRESS` needs sufficient ETH to cover gas fees
- Transaction requires ~21000 gas * maxFeePerGas

### "nonce too low"

- A transaction with this nonce was already sent
- Wait for the previous transaction to confirm, or increase the nonce manually

## API Endpoint Reference

### POST /propose

Proposes a transaction for signing by the plugin agent. This endpoint performs synchronous signing using the verifier or plugin emitter.

**Query Parameters:**
- `policy_id` (required): UUID of the plugin policy
- `network` (required): Network identifier (e.g., "Ethereum", "BSC")
- `tx_hex` (required): Hex-encoded unsigned transaction (without 0x prefix)

**Response:**
```json
{
  "policy_id": "12345678-1234-1234-1234-123456789abc",
  "network": "Ethereum",
  "tx_hex": "02ea0114843b9aca00...",
  "signature": {
    "r": "456d92c1184a652a16f80ae3b371cad591a82855b0eac7fbfd481a5259cd8901",
    "s": "0c15db4ff27930a27476e322d44d033dfbb7ab18abe5479bd81f90abd7100812",
    "recovery_id": "1",
    "r_bytes": null,
    "s_bytes": null,
    "der_signature": null
  }
}
```

## Implementation Details

The integration test consists of three main components:

### IntegrationTest Struct
- Manages connections to agent server and Ethereum RPC
- Holds policy ID and address configuration
- Provides HTTP client with 30-second timeout

### Test Flow

1. **GenerateUnsignedTx**: Creates EIP-1559 (type 0x02) transaction
   - Fetches current nonce from Ethereum RPC
   - Sets gas parameters (maxPriorityFee: 1 Gwei, maxFee: 20 Gwei, limit: 21000)
   - RLP encodes unsigned transaction payload
   - Returns hex-encoded transaction without 0x prefix

2. **ProposeTransaction**: Submits to `/propose` endpoint
   - Sends policy_id, network, and tx_hex as query parameters
   - Agent performs policy validation and signs synchronously
   - Returns signature with r, s, and recovery_id

3. **BroadcastTransaction**: Broadcasts to Ethereum mainnet
   - Decodes unsigned transaction from hex
   - Reconstructs DynamicFeeTx with signature (V, R, S)
   - Sends to Ethereum network via ethclient
   - Returns transaction hash and Etherscan link

## Notes

- Uses EIP-1559 transactions (type 0x02) with dynamic fees
- Signatures are ECDSA with recovery ID (0 or 1)
- All hex values in API requests/responses use no "0x" prefix
- Agent performs policy validation before signing
- Broadcasting requires Ethereum RPC endpoint with write access
- Test timeout: 2 minutes for entire flow