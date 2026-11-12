# POST /releases/{version}   (deprecated)
# The same as POST /releases/gitlab/{version}, with headers pointing to its successor.

POST {{url}}/releases/v0.1.0
HTTP 200
[Asserts]
header "Deprecation" == "true"
header "Link" == "</releases/gitlab/v0.1.0>; rel=\"successor-version\""
jsonpath "$.tag_name" == "v0.1.0"
jsonpath "$.assets" count == 8
jsonpath "$.assets[4].url" == "{{url}}/gitlab/artifacts/v0.1.0/files/gitlab_0.1.0_linux_amd64.tar.gz"

POST {{url}}/releases/latest
HTTP 200
[Asserts]
header "Link" == "</releases/gitlab/latest>; rel=\"successor-version\""
jsonpath "$.tag_name" == "v0.1.0"

# The deprecation headers are sent on errors too.
POST {{url}}/releases/v9.9.9
HTTP 404
[Asserts]
header "Deprecation" == "true"
jsonpath "$.error" == "not found"
