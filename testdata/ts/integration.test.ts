#!/usr/bin/env ts-node

/**
 * Integration test for Vultisig Plugin Agent
 *
 * This test demonstrates the complete flow:
 * 1. Generate unsigned EIP-1559 transaction
 * 2. Propose transaction to agent server for signing
 * 3. Broadcast signed transaction to Ethereum network
 */

import { ethers } from 'ethers';
import axios, { type AxiosInstance } from 'axios';
import type { ProposalResponse, IntegrationTestConfig } from './types';

class IntegrationTest {
  private agentURL: string;
  private ethProvider: ethers.Provider;
  private policyID: string;
  private fromAddress: string;
  private toAddress: string;
  private httpClient: AxiosInstance;

  constructor(config: IntegrationTestConfig) {
    this.agentURL = config.agentURL;
    this.ethProvider = new ethers.JsonRpcProvider(config.ethRPCURL);
    this.policyID = config.policyID;
    this.fromAddress = config.fromAddress;
    this.toAddress = config.toAddress;
    this.httpClient = axios.create({
      timeout: 30000,
    });
  }

  /**
   * Step 1: Generate unsigned EIP-1559 transaction
   */
  async generateUnsignedTx(valueWei: bigint): Promise<string> {
    const nonce = await this.ethProvider.getTransactionCount(this.fromAddress, 'pending');
    const network = await this.ethProvider.getNetwork();
    const chainId = network.chainId;

    const maxPriorityFeePerGas = 100_000_000n;
    const maxFeePerGas = 500_000_000n;
    const gasLimit = 21_000n;

    const tx = ethers.Transaction.from({
      type: 2,
      chainId: chainId,
      nonce: nonce,
      maxPriorityFeePerGas: maxPriorityFeePerGas,
      maxFeePerGas: maxFeePerGas,
      gasLimit: gasLimit,
      to: this.toAddress,
      value: valueWei,
      data: '0x',
      accessList: [],
    });

    const unsignedTx = tx.unsignedSerialized;
    const txHex = unsignedTx.substring(2);

    console.log('[Step 1] Generated unsigned tx:', txHex);
    console.log('         From:', this.fromAddress);
    console.log('         To:', this.toAddress);
    console.log('         Value:', valueWei.toString(), 'wei');
    console.log('         Nonce:', nonce);
    console.log('         Chain ID:', chainId.toString());

    return txHex;
  }

  /**
   * Step 2: Propose transaction to agent server
   */
  async proposeTransaction(txHex: string, network: string): Promise<ProposalResponse> {
    const proposeURL = `${this.agentURL}/propose`;
    const params = new URLSearchParams({
      policy_id: this.policyID,
      network: network,
      tx_hex: txHex,
    });

    console.log('[Step 2] Proposing to:', `${proposeURL}?${params.toString()}`);

    const response = await this.httpClient.post<ProposalResponse>(
      `${proposeURL}?${params.toString()}`
    );

    if (response.status !== 200) {
      throw new Error(`Unexpected status ${response.status}: ${JSON.stringify(response.data)}`);
    }

    const proposalResp = response.data;

    console.log('[Step 2] Received signature:');
    console.log('         R:', proposalResp.signature.r);
    console.log('         S:', proposalResp.signature.s);
    console.log('         RecoveryID:', proposalResp.signature.recovery_id);

    return proposalResp;
  }

  /**
   * Step 3: Broadcast signed transaction to Ethereum network
   */
  async broadcastTransaction(proposalResp: ProposalResponse): Promise<string> {
    const txHex = proposalResp.tx_hex;
    const sig = proposalResp.signature;

    const rawTx = Buffer.from(txHex, 'hex');

    if (rawTx[0] !== 0x02) {
      throw new Error(`Expected type 0x02, got 0x${rawTx[0].toString(16)}`);
    }

    const payload = rawTx.slice(1);
    const decodedTx = ethers.Transaction.from('0x' + txHex);

    const r = '0x' + sig.r;
    const s = '0x' + sig.s;
    const v = sig.recovery_id === '1' ? 1 : 0;

    const signedTx = ethers.Transaction.from({
      type: 2,
      chainId: decodedTx.chainId,
      nonce: decodedTx.nonce,
      maxPriorityFeePerGas: decodedTx.maxPriorityFeePerGas,
      maxFeePerGas: decodedTx.maxFeePerGas,
      gasLimit: decodedTx.gasLimit,
      to: decodedTx.to,
      value: decodedTx.value,
      data: decodedTx.data,
      accessList: decodedTx.accessList,
      signature: ethers.Signature.from({ r, s, v }),
    });

    console.log('[Step 3] Broadcasting transaction...');
    console.log('         Tx Hash:', signedTx.hash);

    const txResponse = await this.ethProvider.broadcastTransaction(signedTx.serialized);

    console.log('[Step 3] Transaction broadcasted successfully!');
    console.log('         Tx Hash:', txResponse.hash);
    console.log('         Explorer: https://etherscan.io/tx/' + txResponse.hash);

    return txResponse.hash;
  }

  /**
   * Run executes the complete integration test
   */
  async run(valueWei: bigint, network: string): Promise<void> {
    console.log('=== Starting Integration Test ===\n');

    const txHex = await this.generateUnsignedTx(valueWei);
    console.log();

    const proposalResp = await this.proposeTransaction(txHex, network);
    console.log();

    const txHash = await this.broadcastTransaction(proposalResp);
    console.log();

    console.log('=== Integration Test Completed Successfully ===');
    console.log('Final Tx Hash:', txHash);
  }
}

async function main() {
  const config: IntegrationTestConfig = {
    agentURL: process.env.AGENT_URL || 'http://localhost:8081',
    ethRPCURL: process.env.ETH_RPC_URL || '',
    policyID: process.env.POLICY_ID || '',
    fromAddress: process.env.FROM_ADDRESS || '',
    toAddress: process.env.TO_ADDRESS || '',
    valueWei: process.env.VALUE_WEI || '1000',
    network: process.env.NETWORK || 'Ethereum',
  };

  if (!config.agentURL) {
    throw new Error('AGENT_URL environment variable is required');
  }
  if (!config.ethRPCURL) {
    throw new Error('ETH_RPC_URL environment variable is required');
  }
  if (!config.policyID) {
    throw new Error('POLICY_ID environment variable is required');
  }
  if (!config.fromAddress) {
    throw new Error('FROM_ADDRESS environment variable is required');
  }
  if (!config.toAddress) {
    throw new Error('TO_ADDRESS environment variable is required');
  }

  console.log('Configuration:');
  console.log('  Agent URL:', config.agentURL);
  console.log('  Ethereum RPC:', config.ethRPCURL);
  console.log('  Policy ID:', config.policyID);
  console.log('  From Address:', config.fromAddress);
  console.log('  To Address:', config.toAddress);
  console.log('  Value:', config.valueWei, 'wei');
  console.log('  Network:', config.network);
  console.log();

  const test = new IntegrationTest(config);

  const valueWei = BigInt(config.valueWei);
  await test.run(valueWei, config.network);
}

if (require.main === module) {
  main().catch((error) => {
    console.error('Test failed:', error);
    process.exit(1);
  });
}

export { IntegrationTest };