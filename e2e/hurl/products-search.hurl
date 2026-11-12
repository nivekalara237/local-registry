# GET /products/search
# Products matching every criterion given, one page at a time (keyset pagination).
# Seeded: billing-db-migrations (published last), gitlab, legacy-tool (published first, deprecated).

# --- No criterion: every product, sorted by name, default limit 25, last page.
GET {{url}}/products/search
HTTP 200
[Asserts]
header "Content-Type" == "application/json"
header "Link" not exists
jsonpath "$.apiVersion" == "local-registry/v1"
jsonpath "$.total" == 3
jsonpath "$.limit" == 25
jsonpath "$.next" == null
jsonpath "$.items" count == 3
jsonpath "$.items[0].product" == "billing-db-migrations"
jsonpath "$.items[1].product" == "gitlab"
jsonpath "$.items[2].product" == "legacy-tool"

# --- An item: the fields of GET /products, plus keywords, labels, license and deprecated.
GET {{url}}/products/search
[Query]
q: gitlab
HTTP 200
[Asserts]
jsonpath "$.total" == 1
jsonpath "$.items[0].product" == "gitlab"
jsonpath "$.items[0].displayName" == "GitLab CLI"
jsonpath "$.items[0].description" == "Command line client for the internal GitLab."
jsonpath "$.items[0].latest" == "v0.1.0"
jsonpath "$.items[0].updatedAt" isIsoDate
jsonpath "$.items[0].keywords" count == 2
jsonpath "$.items[0].labels.team" == "platform"
jsonpath "$.items[0].license" == "Apache-2.0"
jsonpath "$.items[0].deprecated" == false

# Empty keywords and labels are [] and {}, never absent.
GET {{url}}/products/search
[Query]
q: legacy
HTTP 200
[Asserts]
jsonpath "$.items[0].keywords" count == 1
jsonpath "$.items[0].labels" isObject
jsonpath "$.items[0].labels" isEmpty
jsonpath "$.items[0].license" == ""

# --- Pagination: limit 1, follow next until null. The Link header carries the same cursor.
GET {{url}}/products/search
[Query]
limit: 1
HTTP 200
[Captures]
next1: jsonpath "$.next"
[Asserts]
jsonpath "$.total" == 3
jsonpath "$.limit" == 1
jsonpath "$.items" count == 1
jsonpath "$.items[0].product" == "billing-db-migrations"
jsonpath "$.next" isString
header "Link" startsWith "</products/search?"
header "Link" contains "limit=1"
header "Link" endsWith ">; rel=\"next\""

GET {{url}}/products/search
[Query]
limit: 1
cursor: {{next1}}
HTTP 200
[Captures]
next2: jsonpath "$.next"
[Asserts]
jsonpath "$.total" == 3
jsonpath "$.items[0].product" == "gitlab"
jsonpath "$.next" isString

# limit may change from page to page.
GET {{url}}/products/search
[Query]
limit: 5
cursor: {{next2}}
HTTP 200
[Asserts]
jsonpath "$.items" count == 1
jsonpath "$.items[0].product" == "legacy-tool"
jsonpath "$.next" == null
header "Link" not exists

# --- Sorts.
GET {{url}}/products/search
[Query]
sort: updatedAt
HTTP 200
[Asserts]
jsonpath "$.items[0].product" == "legacy-tool"
jsonpath "$.items[1].product" == "gitlab"
jsonpath "$.items[2].product" == "billing-db-migrations"

GET {{url}}/products/search
[Query]
sort: -updatedAt
limit: 2
HTTP 200
[Captures]
recent: jsonpath "$.next"
[Asserts]
jsonpath "$.items[0].product" == "billing-db-migrations"
jsonpath "$.items[1].product" == "gitlab"

GET {{url}}/products/search
[Query]
sort: -updatedAt
limit: 2
cursor: {{recent}}
HTTP 200
[Asserts]
jsonpath "$.items" count == 1
jsonpath "$.items[0].product" == "legacy-tool"

GET {{url}}/products/search
[Query]
sort: -product
HTTP 200
[Asserts]
jsonpath "$.items[0].product" == "legacy-tool"
jsonpath "$.items[2].product" == "billing-db-migrations"

GET {{url}}/products/search
[Query]
sort: -displayName
HTTP 200
[Asserts]
jsonpath "$.items[0].displayName" == "legacy-tool"
jsonpath "$.items[1].displayName" == "GitLab CLI"
jsonpath "$.items[2].displayName" == "Billing DB migrations"

# --- Filters. q: any case, in the name, display name, description and keywords.
GET {{url}}/products/search
[Query]
q: GITLAB
HTTP 200
[Asserts]
jsonpath "$.total" == 1
jsonpath "$.items[0].product" == "gitlab"

GET {{url}}/products/search
[Query]
q: liquibase
HTTP 200
[Asserts]
jsonpath "$.total" == 1
jsonpath "$.items[0].product" == "billing-db-migrations"

GET {{url}}/products/search
[Query]
q: old bridge
HTTP 200
[Asserts]
jsonpath "$.items[0].product" == "legacy-tool"

# keyword: any case, repeated keywords must all match.
GET {{url}}/products/search
[Query]
keyword: CLI
HTTP 200
[Asserts]
jsonpath "$.total" == 2
jsonpath "$.items[0].product" == "gitlab"
jsonpath "$.items[1].product" == "legacy-tool"

GET {{url}}/products/search
[Query]
keyword: cli
keyword: gitlab
HTTP 200
[Asserts]
jsonpath "$.total" == 1
jsonpath "$.items[0].product" == "gitlab"

# label: key=value, exact; numbers of registry.yaml are strings. Repeated labels must all match.
GET {{url}}/products/search
[Query]
label: team=billing
label: cost-center=4512
HTTP 200
[Asserts]
jsonpath "$.total" == 1
jsonpath "$.items[0].product" == "billing-db-migrations"

GET {{url}}/products/search
[Query]
label: team=Billing
HTTP 200
[Asserts]
jsonpath "$.total" == 0
jsonpath "$.items" isEmpty
jsonpath "$.next" == null

# owner: name or email, any case.
GET {{url}}/products/search
[Query]
owner: platform team
HTTP 200
[Asserts]
jsonpath "$.items[0].product" == "gitlab"

GET {{url}}/products/search
[Query]
owner: BILLING@corp.example
HTTP 200
[Asserts]
jsonpath "$.total" == 1
jsonpath "$.items[0].product" == "billing-db-migrations"

# license: any case.
GET {{url}}/products/search
[Query]
license: apache-2.0
HTTP 200
[Asserts]
jsonpath "$.total" == 1
jsonpath "$.items[0].product" == "gitlab"

# deprecated: of the current version.
GET {{url}}/products/search
[Query]
deprecated: true
HTTP 200
[Asserts]
jsonpath "$.total" == 1
jsonpath "$.items[0].product" == "legacy-tool"
jsonpath "$.items[0].deprecated" == true

GET {{url}}/products/search
[Query]
deprecated: false
HTTP 200
[Asserts]
jsonpath "$.total" == 2

# prerelease: of the current version. gitlab has a pre-release, but its latest v0.1.0 is stable.
GET {{url}}/products/search
[Query]
prerelease: true
HTTP 200
[Asserts]
jsonpath "$.total" == 0

GET {{url}}/products/search
[Query]
prerelease: false
HTTP 200
[Asserts]
jsonpath "$.total" == 3

# updatedSince: inclusive, YYYY-MM-DD.
GET {{url}}/products/search
[Query]
updatedSince: 2000-01-01
HTTP 200
[Asserts]
jsonpath "$.total" == 3

GET {{url}}/products/search
[Query]
updatedSince: 2999-01-01
HTTP 200
[Asserts]
jsonpath "$.total" == 0

# Criteria combined: every one must match.
GET {{url}}/products/search
[Query]
keyword: cli
deprecated: false
license: Apache-2.0
HTTP 200
[Asserts]
jsonpath "$.total" == 1
jsonpath "$.items[0].product" == "gitlab"

# --- Errors: 400 with every invalid parameter in one message.
GET {{url}}/products/search
[Query]
foo: bar
HTTP 400
[Asserts]
jsonpath "$.error" == "unknown parameter \"foo\": use q, keyword, label, owner, license, deprecated, prerelease, updatedSince, sort, limit, cursor"

GET {{url}}/products/search
[Query]
deprecated: maybe
prerelease: 2
label: team
sort: name
updatedSince: 2026-13-01
limit: 101
HTTP 400
[Asserts]
jsonpath "$.error" contains "deprecated must be true or false"
jsonpath "$.error" contains "prerelease must be true or false"
jsonpath "$.error" contains "label \"team\" must be key=value"
jsonpath "$.error" contains "sort must be one of [product displayName updatedAt]"
jsonpath "$.error" contains "updatedSince must be a date: YYYY-MM-DD"
jsonpath "$.error" contains "limit must be between 1 and 100"

GET {{url}}/products/search
[Query]
limit: 0
HTTP 400
[Asserts]
jsonpath "$.error" == "limit must be between 1 and 100"

GET {{url}}/products/search
[Query]
cursor: not-a-cursor
HTTP 400
[Asserts]
jsonpath "$.error" == "invalid cursor"

# A cursor is bound to its criteria and sort: reused with others, it is refused.
GET {{url}}/products/search
[Query]
limit: 1
HTTP 200
[Captures]
by_name: jsonpath "$.next"

GET {{url}}/products/search
[Query]
limit: 1
sort: -updatedAt
cursor: {{by_name}}
HTTP 400
[Asserts]
jsonpath "$.error" == "cursor does not match the query: keep the same filters and sort"

GET {{url}}/products/search
[Query]
limit: 1
keyword: cli
cursor: {{by_name}}
HTTP 400
