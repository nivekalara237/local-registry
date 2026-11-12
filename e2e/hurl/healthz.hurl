# GET /healthz
# Liveness probe: always 200, whatever the state of the disk.

GET {{url}}/healthz
HTTP 200
[Asserts]
header "Content-Type" == "application/json"
jsonpath "$.status" == "ok"

# Only GET is routed.
POST {{url}}/healthz
HTTP 405
