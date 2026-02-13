# Go Integration Test

Complete end-to-end test: generate tx → propose → broadcast to Ethereum.

## Run

```bash
cp .env.example .env
# Edit .env with your values
go run integration.go
```

## Environment

```bash
AGENT_URL=http://localhost:8081
ETH_RPC_URL=https://ethereum-rpc.publicnode.com
POLICY_ID=your-policy-uuid
FROM_ADDRESS=0x...
TO_ADDRESS=0x...
VALUE_WEI=1000
NETWORK=Ethereum
```

## Output

```
=== Starting Integration Test ===

[Step 1] Generated unsigned tx: 02ea0114...
[Step 2] Received signature: R, S, RecoveryID
[Step 3] Transaction broadcasted: 0x...

=== Integration Test Completed Successfully ===
```

## Troubleshooting

- **"failed to get plugin policy"** - Policy ID doesn't exist
- **"failed to sign request"** - Worker not running or vault not configured
- **"tx target is wrong"** - TO_ADDRESS doesn't match policy rule