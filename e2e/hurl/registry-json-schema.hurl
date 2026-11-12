# GET /registry/json-schema
# The JSON Schema of registry.yaml, as shipped in schema/registry.schema.json.

GET {{url}}/registry/json-schema
HTTP 200
[Asserts]
header "Content-Type" == "application/json"
jsonpath "$['$schema']" == "https://json-schema.org/draft/2020-12/schema"
jsonpath "$.title" contains "registry.yaml"
jsonpath "$.required" contains "apiVersion"
jsonpath "$.properties.apiVersion.const" == "local-registry/v1"
jsonpath "$.additionalProperties" == false
