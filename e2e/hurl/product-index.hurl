# GET /{product}/index
# index.json of a product: its card (from the latest version) and every version, newest first.

GET {{url}}/gitlab/index
HTTP 200
[Asserts]
header "Content-Type" == "application/json"
jsonpath "$.apiVersion" == "local-registry/v1"
jsonpath "$.product" == "gitlab"
jsonpath "$.displayName" == "GitLab CLI"
jsonpath "$.license" == "Apache-2.0"
jsonpath "$.owners[0].name" == "Platform team"
jsonpath "$.owners[0].email" == "platform@corp.example"
jsonpath "$.links[0].name" == "source"
jsonpath "$.links[0].url" == "https://git.corp.example/platform/gitlab-cli"
jsonpath "$.keywords" contains "gitlab"
jsonpath "$.labels.team" == "platform"
jsonpath "$.install" startsWith "tar xzf"
jsonpath "$.latest" == "v0.1.0"
jsonpath "$.updatedAt" isIsoDate
jsonpath "$.versions" count == 2
# Highest version first: the pre-release v0.2.0-rc.1 is above v0.1.0, even though not latest.
jsonpath "$.versions[0].version" == "v0.2.0-rc.1"
jsonpath "$.versions[0].prerelease" == true
jsonpath "$.versions[0].tag" == null
jsonpath "$.versions[1].version" == "v0.1.0"
jsonpath "$.versions[1].tag" == "v0.1.0"
jsonpath "$.versions[1].prerelease" == false
jsonpath "$.versions[1].deprecated" == false
jsonpath "$.versions[1].securityUpdates" == false
jsonpath "$.versions[1].publishedAt" isIsoDate

# deprecated: a message in registry.yaml is reported in the version entry.
GET {{url}}/legacy-tool/index
HTTP 200
[Asserts]
jsonpath "$.latest" == "v0.9.0"
jsonpath "$.versions" count == 1
jsonpath "$.versions[0].deprecated" == "replaced by gitlab"

# Unknown product.
GET {{url}}/missing/index
HTTP 404
[Asserts]
jsonpath "$.error" == "not found"

# "products" is reserved: it is not a product.
GET {{url}}/products/index
HTTP 404
