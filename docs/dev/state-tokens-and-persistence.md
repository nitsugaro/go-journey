# State, tokens, and persistence

This page documents the resumable-state contract. Read it before changing token claims, encrypted context, server-side state storage, TTL behavior, or replay protection.

## Split-state model

Go Journey deliberately splits resumable state:

- The signed JWT carries tracking IDs, JTI, public context, and encrypted context.
- The state store carries closed context, callbacks, restored map state, and replay-control ownership.

The token alone is insufficient. A valid signature with a missing or already-consumed JTI is rejected.

## Token flow

On pause:

1. Generate a fresh random JTI.
2. Serialize and encrypt contexts.
3. Sign a token containing the resumable claims.
4. Store server state under the same JTI.

On resume:

1. Parse protected headers and load the signing key.
2. Verify the HS256 signature.
3. Deserialize claims.
4. Validate the tracking frame.
5. Atomically `GetAndDelete(JTI)`.
6. Merge server state.

Invalid client input restores the consumed state so the same signed request can be corrected. Other execution errors are terminal unless explicitly handled by a step.

## Suspended state

SuspendFlow uses a random URL-safe resume ID instead of a JWT. The ID is stored in temporary context during execution, then becomes the state key. Journey and step identifiers remain in closed context. The resume TTL comes from `SuspendFlow.exp` seconds plus any consumed extension.

## Storage contract

`JourneyStates.GetAndDelete` must be atomic across all replicas. A Redis implementation should use a transaction or Lua script; a database implementation should use a deleting `RETURNING` operation or equivalent locked transaction.

`Store` must return false when persistence fails. The engine converts that into `ErrJourneyStateStore`.

The built-in store:

- Is in-memory.
- Is protected for concurrent access.
- Supports per-entry TTL.
- Does not survive restart.
- Must not be used across multiple server replicas.

## Encryption

Encrypted context uses AES-GCM. The ciphertext is also covered by the journey-token signature.

Manager key rules:

- Valid lengths: 16, 24, or 32 bytes.
- Copy the key into manager-owned memory during construction.
- Use the same key anywhere shared state may resume.
- Rotate only with a migration strategy for outstanding tokens.
- Treat decryption failure as an invalid token; never silently replace confidential state with an empty map.

## Expiration semantics

`default_exp` is measured in minutes and used as an idle TTL whenever ordinary paused state is stored. ExtendExp places a minute delta in context; persistence consumes and removes it before serialization. SuspendFlow uses its seconds-based expiration instead.

Be careful when changing expiration semantics: callers may rely on idle extension across a sequence of client interactions.
