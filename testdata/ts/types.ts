/**
 * Type definitions for Vultisig Plugin Agent API
 */

export interface KeysignResponse {
  r: string;
  s: string;
  recovery_id: string;
  r_bytes?: number[] | null;
  s_bytes?: number[] | null;
  der_signature?: number[] | null;
}

export interface ProposalResponse {
  policy_id: string;
  network: string;
  tx_hex: string;
  signature: KeysignResponse;
}

export interface DeriveAddressResponse {
  address: string;
  public_key: string;
  curve_type: string;
  root_public_key: string;
  chain_code: string;
}

export interface ErrorResponse {
  message: string;
}

export interface WebSocketMessage {
  type: string;
  data?: any;
}

export interface SubscriptionRequest {
  channel: string;
  last_seen?: number;
}

export interface EventMessage {
  id: number;
  public_key?: string;
  policy_id?: string;
  event_type: string;
  event_data: any;
  created_at: string;
}

export interface IntegrationTestConfig {
  agentURL: string;
  ethRPCURL: string;
  policyID: string;
  fromAddress: string;
  toAddress: string;
  valueWei: string;
  network: string;
}