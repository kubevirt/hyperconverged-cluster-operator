package golden_images

import _ "embed"

//go:embed testFiles/dataImportCronTemplates/dataImportCronTemplates.yaml
var validDataImportCronFileContent []byte

//go:embed testFiles/dataImportCronTemplates/wrongDataImportCronTemplates.yaml
var wrongDataImportCronTemplatesFileContent []byte

//go:embed testFiles/dataImportCronTemplates/dataImportCronTemplatesWithImageStream.yaml
var validDataImportCronWithImageStreamFileContent []byte
