# Seed 2/3: gitlab v0.1.0, published from a goreleaser dist and made latest, then
# v0.2.0-rc.1, a pre-release published in folder mode and not made latest.
# Each publish waits 1 s so that the products have distinct publishedAt (to the second).

PUT {{url}}/gitlab/artifacts/v0.1.0/files/registry.yaml
Authorization: Bearer {{token}}
X-Checksum-Sha256: ecefee6ea38f8be32378808d0d88c28f7e97c81b1af88252ce61975d5046cca7
file,fixtures/gitlab/v0.1.0/registry.yaml;
HTTP 201
[Asserts]
jsonpath "$.name" == "registry.yaml"
jsonpath "$.sha256" == "ecefee6ea38f8be32378808d0d88c28f7e97c81b1af88252ce61975d5046cca7"

PUT {{url}}/gitlab/artifacts/v0.1.0/files/gitlab_0.1.0_linux_amd64.tar.gz
Authorization: Bearer {{token}}
X-Checksum-Sha256: 5e3c935149d6200dc6a2796e4839e9b6c0a4fd05368b83be59ba4e461edd7683
file,fixtures/gitlab/v0.1.0/gitlab_0.1.0_linux_amd64.tar.gz;
HTTP 201
[Asserts]
jsonpath "$.name" == "gitlab_0.1.0_linux_amd64.tar.gz"
jsonpath "$.sha256" == "5e3c935149d6200dc6a2796e4839e9b6c0a4fd05368b83be59ba4e461edd7683"

PUT {{url}}/gitlab/artifacts/v0.1.0/files/gitlab_0.1.0_darwin_arm64.tar.gz
Authorization: Bearer {{token}}
X-Checksum-Sha256: 8ce6a6ede1b221e225d57e882740143ad598fa99c5493296086466b9f119f08f
file,fixtures/gitlab/v0.1.0/gitlab_0.1.0_darwin_arm64.tar.gz;
HTTP 201
[Asserts]
jsonpath "$.name" == "gitlab_0.1.0_darwin_arm64.tar.gz"
jsonpath "$.sha256" == "8ce6a6ede1b221e225d57e882740143ad598fa99c5493296086466b9f119f08f"

PUT {{url}}/gitlab/artifacts/v0.1.0/files/gitlab_0.1.0_checksums.txt
Authorization: Bearer {{token}}
X-Checksum-Sha256: b1a61d53873cc29e3fe4218ae72938fb5dcc9e6af902e33c71a4cb06f6a3730d
file,fixtures/gitlab/v0.1.0/gitlab_0.1.0_checksums.txt;
HTTP 201
[Asserts]
jsonpath "$.name" == "gitlab_0.1.0_checksums.txt"
jsonpath "$.sha256" == "b1a61d53873cc29e3fe4218ae72938fb5dcc9e6af902e33c71a4cb06f6a3730d"

PUT {{url}}/gitlab/artifacts/v0.1.0/files/metadata.json
Authorization: Bearer {{token}}
X-Checksum-Sha256: 471377929f0abc1c161d5738a78359da735ae34c20e9b50d8c027bb012a5f2de
file,fixtures/gitlab/v0.1.0/metadata.json;
HTTP 201
[Asserts]
jsonpath "$.name" == "metadata.json"
jsonpath "$.sha256" == "471377929f0abc1c161d5738a78359da735ae34c20e9b50d8c027bb012a5f2de"

PUT {{url}}/gitlab/artifacts/v0.1.0/files/artifacts.json
Authorization: Bearer {{token}}
X-Checksum-Sha256: 85ef0db8285b505eef462e91414d0aa6f91bcd79771183071bcf1b8287c1bd8f
file,fixtures/gitlab/v0.1.0/artifacts.json;
HTTP 201
[Asserts]
jsonpath "$.name" == "artifacts.json"
jsonpath "$.sha256" == "85ef0db8285b505eef462e91414d0aa6f91bcd79771183071bcf1b8287c1bd8f"

POST {{url}}/gitlab/artifacts/v0.1.0/publish?latest=true
Authorization: Bearer {{token}}
[Options]
delay: 1100ms
HTTP 200
[Asserts]
jsonpath "$.latest" == true
# goreleaser mode: the uploaded artifacts.json is kept as artifacts.goreleaser.json.
jsonpath "$.files" count == 8
jsonpath "$.files" contains "artifacts.goreleaser.json"
jsonpath "$.files" contains "gitlab_0.1.0_checksums.txt"
jsonpath "$.files" contains "release.json"
jsonpath "$.files" not contains "checksums.txt"

PUT {{url}}/gitlab/artifacts/v0.2.0-rc.1/files/registry.yaml
Authorization: Bearer {{token}}
X-Checksum-Sha256: 06f3d467a8a4714520378b427f260f8d46c4c7c9412e19bc47d027d235248381
file,fixtures/gitlab/v0.2.0-rc.1/registry.yaml;
HTTP 201
[Asserts]
jsonpath "$.name" == "registry.yaml"
jsonpath "$.sha256" == "06f3d467a8a4714520378b427f260f8d46c4c7c9412e19bc47d027d235248381"

PUT {{url}}/gitlab/artifacts/v0.2.0-rc.1/files/gitlab_0.2.0-rc.1_linux_amd64.tar.gz
Authorization: Bearer {{token}}
X-Checksum-Sha256: f7906c4a97f4a41de14816549bdf7734379ff2ca6d0de0e16ac1da21d3f3a645
file,fixtures/gitlab/v0.2.0-rc.1/gitlab_0.2.0-rc.1_linux_amd64.tar.gz;
HTTP 201
[Asserts]
jsonpath "$.name" == "gitlab_0.2.0-rc.1_linux_amd64.tar.gz"
jsonpath "$.sha256" == "f7906c4a97f4a41de14816549bdf7734379ff2ca6d0de0e16ac1da21d3f3a645"

POST {{url}}/gitlab/artifacts/v0.2.0-rc.1/publish
Authorization: Bearer {{token}}
HTTP 200
[Asserts]
jsonpath "$.latest" == false
jsonpath "$.version" == "v0.2.0-rc.1"
