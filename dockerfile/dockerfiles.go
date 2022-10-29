package dockerfile

import _ "embed"

//go:embed sdk/Dockerfile
var SDK []byte

//go:embed sdk/native.Dockerfile
var SDKNative []byte

//go:embed sdk/local.Dockerfile
var SDKLocal []byte

//go:embed imported/Dockerfile
var Imported []byte

//go:embed none/Dockerfile
var None []byte

//go:embed rust/Dockerfile
var Rust []byte

//go:embed rust/native.Dockerfile
var RustNative []byte

//go:embed nix/Dockerfile
var Nix []byte

//go:embed nix/native.Dockerfile
var NixNative []byte
