# Veriqid JS SDK (Phase 8)

Drop-in JavaScript SDK for platform developers.

## Planned API

```js
Veriqid.init({ bridgeUrl: 'https://...' })
Veriqid.generateChallenge()
Veriqid.verifyRegistration(proof, spk, challenge)
Veriqid.verifyAuth(signature, spk, challenge)
```

Starting point: `crypto-snark/src/index.js` from the U2SSO repo.
Note: Uses poseidon2 for web keys, poseidon3 for child/SPK keys (count=100).
