# TypeScript Integration Test

Same as Go test but in TypeScript.

## Setup

```bash
npm install
cp .env.example .env
# Edit .env
```

## Run

```bash
npm run test
```

## Environment

Same as Go test - see `.env.example`

## Files

- `integration.test.ts` - Full E2E test (same as Go)
- `types.ts` - TypeScript types
- `package.json` - NPM config with dotenv
- `tsconfig.json` - TypeScript config

## Notes

- Uses `dotenv` to auto-load `.env` file
- Port 8081 for agent (8080 is verifier)
- TO_ADDRESS must match policy rule
