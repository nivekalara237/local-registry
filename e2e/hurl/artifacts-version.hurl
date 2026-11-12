# GET /{product}/artifacts/{version}
# artifacts.json of one version; "latest" behaves like GET /{product}/artifacts.

GET {{url}}/gitlab/artifacts/v0.1.0
HTTP 200
[Asserts]
jsonpath "$" count == 4
jsonpath "$[1].path" == "{{url}}/gitlab/artifacts/v0.1.0/files/gitlab_0.1.0_linux_amd64.tar.gz"

GET {{url}}/gitlab/artifacts/latest
HTTP 200
[Asserts]
jsonpath "$" count == 4
jsonpath "$[0].path" contains "/v0.1.0/"

# Published in folder mode: no artifacts.json.
GET {{url}}/gitlab/artifacts/v0.2.0-rc.1
HTTP 404
[Asserts]
jsonpath "$.error" == "not found"

GET {{url}}/gitlab/artifacts/v9.9.9
HTTP 404

GET {{url}}/missing/artifacts/v0.1.0
HTTP 404
