# Seed 3/3: billing-db-migrations v1.0.0, published in folder mode with a CHANGELOG.md, made latest.

PUT {{url}}/billing-db-migrations/artifacts/v1.0.0/files/registry.yaml
Authorization: Bearer {{token}}
X-Checksum-Sha256: e89bcf309bce1c70f68332f943db2408bdd511becf7e1f833240a42460b418f7
file,fixtures/billing-db-migrations/v1.0.0/registry.yaml;
HTTP 201
[Asserts]
jsonpath "$.name" == "registry.yaml"
jsonpath "$.sha256" == "e89bcf309bce1c70f68332f943db2408bdd511becf7e1f833240a42460b418f7"

PUT {{url}}/billing-db-migrations/artifacts/v1.0.0/files/CHANGELOG.md
Authorization: Bearer {{token}}
X-Checksum-Sha256: 3fa039e41e5627d53aacbc36f6e3f125f973b2674cd48e63300dd816ec30ddf4
file,fixtures/billing-db-migrations/v1.0.0/CHANGELOG.md;
HTTP 201
[Asserts]
jsonpath "$.name" == "CHANGELOG.md"
jsonpath "$.sha256" == "3fa039e41e5627d53aacbc36f6e3f125f973b2674cd48e63300dd816ec30ddf4"

PUT {{url}}/billing-db-migrations/artifacts/v1.0.0/files/billing-db-migrations_1.0.0.tar.gz
Authorization: Bearer {{token}}
X-Checksum-Sha256: cde6915139e6cf8cf66b69977ae8b8b3bb6b5ed911f87faa802982f12dad9a21
file,fixtures/billing-db-migrations/v1.0.0/billing-db-migrations_1.0.0.tar.gz;
HTTP 201
[Asserts]
jsonpath "$.name" == "billing-db-migrations_1.0.0.tar.gz"
jsonpath "$.sha256" == "cde6915139e6cf8cf66b69977ae8b8b3bb6b5ed911f87faa802982f12dad9a21"

POST {{url}}/billing-db-migrations/artifacts/v1.0.0/publish?latest=true
Authorization: Bearer {{token}}
[Options]
delay: 1100ms
HTTP 200
[Asserts]
jsonpath "$.latest" == true
jsonpath "$.files" contains "CHANGELOG.md"
jsonpath "$.files" contains "checksums.txt"
