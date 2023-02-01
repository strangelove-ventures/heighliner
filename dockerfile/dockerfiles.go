package dockerfile

import _ "embed"

//go:embed cosmos/Dockerfile
var Cosmos []byte

//go:embed cosmos/native.Dockerfile
var CosmosNative []byte

//go:embed cosmos/local.Dockerfile
var CosmosLocal []byte

//go:embed imported/Dockerfile
var Imported []byte

//go:embed none/Dockerfile
var None []byte

//go:embed cargo/Dockerfile
var Cargo []byte

//go:embed cargo/native.Dockerfile
var CargoNative []byte
