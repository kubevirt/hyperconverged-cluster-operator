package upgradepatch

import _ "embed"

//go:embed test-files/upgradePatches.json
var upgradePatchesFileContent []byte

//go:embed test-files/badJson.json
var badJsonFileContent []byte

//go:embed test-files/badObject1.json
var badObject1FileContent []byte

//go:embed test-files/badObject1m.json
var badObject1mFileContent []byte

//go:embed test-files/badObject2.json
var badObject2FileContent []byte

//go:embed test-files/badObject2m.json
var badObject2mFileContent []byte

//go:embed test-files/badObject3.json
var badObject3FileContent []byte

//go:embed test-files/badObject3m.json
var badObject3mFileContent []byte

//go:embed test-files/badPatches1.json
var badPatches1FileContent []byte

//go:embed test-files/badPatches2.json
var badPatches2FileContent []byte

//go:embed test-files/badPatches3.json
var badPatches3FileContent []byte

//go:embed test-files/badPatches4.json
var badPatches4FileContent []byte

//go:embed test-files/badPatches5.json
var badPatches5FileContent []byte

//go:embed test-files/badPatches6.json
var badPatches6FileContent []byte

//go:embed test-files/badPatches7.json
var badPatches7FileContent []byte

//go:embed test-files/badSemverRange.json
var badSemverRangeFileContent []byte

//go:embed test-files/badSemverRangeOR.json
var badSemverRangeORFileContent []byte

//go:embed test-files/empty.json
var emptyFileContent []byte
