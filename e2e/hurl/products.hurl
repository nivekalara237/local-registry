# GET /products
# Every product with its latest version, sorted by name. Seeded: billing-db-migrations,
# gitlab (v0.1.0 latest, v0.2.0-rc.1 pre-release), legacy-tool (deprecated).

GET {{url}}/products
HTTP 200
[Asserts]
header "Content-Type" == "application/json"
jsonpath "$" count == 3
jsonpath "$[0].product" == "billing-db-migrations"
jsonpath "$[1].product" == "gitlab"
jsonpath "$[2].product" == "legacy-tool"

# Fields come from the registry.yaml of the latest version.
jsonpath "$[1].displayName" == "GitLab CLI"
jsonpath "$[1].description" == "Command line client for the internal GitLab."
jsonpath "$[1].latest" == "v0.1.0"
jsonpath "$[1].updatedAt" isIsoDate

# Without displayName in registry.yaml nor metadata.json, the product name is used.
jsonpath "$[2].displayName" == "legacy-tool"

# GET /products only: keywords, labels and owners are in /products/search and /{product}/index.
jsonpath "$[1].keywords" not exists
