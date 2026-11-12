# Seed 1/3: legacy-tool v0.9.0, a deprecated product published in folder mode (no artifacts.json).
# The scenarios read it: keep it in line with the assertions that use it.

PUT {{url}}/legacy-tool/artifacts/v0.9.0/files/registry.yaml
Authorization: Bearer {{token}}
X-Checksum-Sha256: 41f78b00b10f8027c253eb43451ddefed7db93bb872f5b1732af7ca0ecfee314
file,fixtures/legacy-tool/v0.9.0/registry.yaml;
HTTP 201
[Asserts]
jsonpath "$.name" == "registry.yaml"
jsonpath "$.sha256" == "41f78b00b10f8027c253eb43451ddefed7db93bb872f5b1732af7ca0ecfee314"

PUT {{url}}/legacy-tool/artifacts/v0.9.0/files/legacy-tool_0.9.0_linux_amd64.tar.gz
Authorization: Bearer {{token}}
X-Checksum-Sha256: 10ade9f4b2df471204b432931aca8d7dafa40f45c21047ca00776339160154ab
file,fixtures/legacy-tool/v0.9.0/legacy-tool_0.9.0_linux_amd64.tar.gz;
HTTP 201
[Asserts]
jsonpath "$.name" == "legacy-tool_0.9.0_linux_amd64.tar.gz"
jsonpath "$.sha256" == "10ade9f4b2df471204b432931aca8d7dafa40f45c21047ca00776339160154ab"

POST {{url}}/legacy-tool/artifacts/v0.9.0/publish?latest=true
Authorization: Bearer {{token}}
HTTP 200
[Asserts]
jsonpath "$.latest" == true
jsonpath "$.product" == "legacy-tool"
jsonpath "$.version" == "v0.9.0"
# Folder mode: publish writes checksums.txt and release.json.
jsonpath "$.files" count == 4
jsonpath "$.files[0]" == "checksums.txt"
jsonpath "$.files[1]" == "legacy-tool_0.9.0_linux_amd64.tar.gz"
jsonpath "$.files[2]" == "registry.yaml"
jsonpath "$.files[3]" == "release.json"
