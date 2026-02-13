# Integration Tests

Integration tests for Vultisig Plugin Agent in Go and TypeScript.

## Structure

```
testdata/
├── go/                    # Go integration test
│   ├── .env.example
│   ├── integration.go
│   └── README.md
└── ts/                    # TypeScript integration test
    ├── .env.example
    ├── integration.test.ts
    ├── types.ts
    ├── package.json
    ├── tsconfig.json
    └── README.md
```

## Go

```bash
cd go
cp .env.example .env
# Edit .env
go run integration.go
```

## TypeScript

```bash
cd ts
npm install
cp .env.example .env
# Edit .env
npm run test
```

## Environment

Required in `.env` file:

```bash
AGENT_URL=http://localhost:8081
ETH_RPC_URL=https://ethereum-rpc.publicnode.com
POLICY_ID=your-policy-uuid
FROM_ADDRESS=0x...
TO_ADDRESS=0x...
VALUE_WEI=1000
NETWORK=Ethereum
```

## Notes

- Plugin agent: port **8081** (verifier: 8080)
- Test flow: generate tx → propose → broadcast
- TO_ADDRESS must match policy rule
