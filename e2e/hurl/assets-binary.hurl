# GET /assets/binary/{filePath}
# A public file by its path relative to the asset path, encoded as one segment (/ as %2F).
# Only bin.local/assets/... and assets/... are served; anything else answers 404 in plain text.

GET {{url}}/assets/binary/bin.local%2Fassets%2Fbinaries%2Fgitlab%2Fv0.1.0%2Fgitlab_0.1.0_linux_amd64.tar.gz
HTTP 200
[Asserts]
header "Content-Type" == "application/octet-stream"
header "Content-Disposition" == "attachment; filename=gitlab_0.1.0_linux_amd64.tar.gz"
header "Accept-Ranges" == "bytes"
header "Content-Length" == "26"
sha256 == hex,5e3c935149d6200dc6a2796e4839e9b6c0a4fd05368b83be59ba4e461edd7683;

# Files written by publish are public too.
GET {{url}}/assets/binary/bin.local%2Fassets%2Fbinaries%2Fgitlab%2Findex.json
HTTP 200
[Asserts]
bytes count > 0

# Range is supported.
GET {{url}}/assets/binary/bin.local%2Fassets%2Fbinaries%2Fgitlab%2Fv0.1.0%2Fgitlab_0.1.0_linux_amd64.tar.gz
Range: bytes=0-5
HTTP 206
[Asserts]
header "Content-Range" == "bytes 0-5/26"
body == "gitlab"

# Private: the audit log and staging.
GET {{url}}/assets/binary/bin.local%2Faudit.log
HTTP 404
[Asserts]
body == "404 page not found\n"

GET {{url}}/assets/binary/bin.local%2F.uploads%2Fdemo-upload
HTTP 404

# A directory, a missing file, a path leaving the asset path, a non-normalised path.
GET {{url}}/assets/binary/bin.local%2Fassets%2Fbinaries%2Fgitlab
HTTP 404

GET {{url}}/assets/binary/bin.local%2Fassets%2Fbinaries%2Fgitlab%2Fmissing
HTTP 404

GET {{url}}/assets/binary/bin.local%2Fassets%2F..%2F..%2Fetc%2Fpasswd
HTTP 404

GET {{url}}/assets/binary/bin.local%2Fassets%2F%2Fbinaries%2Fgitlab%2Findex.json
HTTP 404

# A segment starting with "." is never served.
GET {{url}}/assets/binary/bin.local%2Fassets%2F.hidden
HTTP 404
