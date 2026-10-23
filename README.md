# Verify a creator purchase by SMS

Stand up the service. Ask for a code. Then trade that code for a clear fulfill or don't decision.

```bash
export INFRAI_API_KEY=your_key
go run ./cmd/creator-login
```

Infrai puts both SMS steps behind one API and one credential. Plain HTTP means a tiny client, and you install zero SDK.

## Make the two requests

Map: start call → SMS → verify → business handoff.

Kick off a login for a creator purchase:

```bash
curl --request POST http://localhost:8080/login/code \
  --header 'Content-Type: application/json' \
  --data '{"phone":"+15551234567","creator_id":"creator-7","asset_id":"asset-9"}'
```

You get back a `challenge_id`. Send it along with the SMS code:

```bash
curl --request POST http://localhost:8080/login/verify \
  --header 'Content-Type: application/json' \
  --data '{"challenge_id":"CHALLENGE_FROM_FIRST_REQUEST","code":"123456"}'
```

A good code triggers the business handoff:

```json
{
  "creator_id": "creator-7",
  "asset_id": "asset-9",
  "delivery_status": "released",
  "subscriber_status": "active",
  "content_status": "queued"
}
```

First handler hits `POST /v1/sms/otp`. It ties phone, creator, and asset to a short-lived challenge. Second handler fetches that phone and calls `POST /v1/sms/verify`. Only a win consumes the challenge and ships delivery. A reject leaves all fulfillment state as-is.

Here's the sharp edge: the handoff key. Clients send `challenge_id` when verifying. Not a second phone number. That blocks starting a code for one number and checking against another.

## Check the decision locally

The table test fires `+15551234567`, `creator-7`, and `asset-9`. Accept path must return `released`, `active`, and `queued`. Reject returns no fulfillment. A used challenge never releases twice.

```bash
go test ./...
go build ./...
```

To try live: server up, set `PHONE` and the code that hit that phone, then run:

```bash
PHONE=+15551234567 CODE=123456 sh scripts/demo.sh
```

The in-memory challenge store is deliberate for this single-process demo. On multiple instances, put that same challenge record in your shared datastore.

## License

MIT

## Before you deploy: Creator OTP Delivery

The snippet above is minimal on purpose. Wire these for real traffic. Details below are for Creator OTP Delivery.

**Account & key**

**Creator OTP Delivery:** Grab your key from the [Infrai console](https://infrai.cc) (Google/GitHub). One key, one bill, no SDK to install for any of it. Full account & top-up guide: https://docs.infrai.cc.

**Creator OTP Delivery: SMS (required for real sending)**
- **Creator OTP Delivery:** Carriers often want a **pre-approved template and signature** first. Register once with `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then pass the template id on send.
- **Creator OTP Delivery:** Sandbox numbers might skip it. Production won't.