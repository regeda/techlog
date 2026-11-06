# Retries and rejected events

Read this when the result of `techlog_record_event` has `isError: true` or says `Already recorded`.

## Retries and idempotency keys

Every event needs an `idempotency_key`. The server stores one event per actor and key, and rejects a second one. This is what makes a retry safe.

- Build the key from the work item, so the same fact always gets the same key: `<repo>#<pr>-merged`, `incident-<ticket>-opened`, `<system>-<version>-deployed`. It is a string of 1 to 128 characters, compared exactly.
- Make the key stable. Never use a random value, a timestamp of the attempt or a counter that restarts, or a retry will not be recognised.
- Make the key specific to one fact. Two different events from the same actor must have different keys, or the second is rejected.
- If the result says `Already recorded: idempotency_key ... is event evt_...`, the first attempt succeeded. Do not retry. Use that id for your links.

## If the server rejects the event

The result has `isError: true` and lists the problems as `- <JSON pointer path>: <message>`. Fix exactly those fields and call the tool again. Common ones:

- `/actor/kind`: must be `human`, `ai_agent` or `system`.
- `/time`: needs a time zone, `Z` or an offset.
- `/summary`: over 280 characters; shorten it.
- `/links/0/target`: that event id does not exist; search again for the right one.
- `/links/0`: `missing property 'basis'`; `caused_by` links must say `confirmed` or `hypothesis`.
- `/`: `missing property 'idempotency_key'`; add the key. `/idempotency_key`: empty or longer than 128 characters.

Do not retry a rejected event unchanged, unless the error was a lost connection: then retry with the same key.
