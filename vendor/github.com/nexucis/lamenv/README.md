Lamenv
======
[![codecov](https://codecov.io/gh/Nexucis/lamenv/branch/master/graph/badge.svg?token=D7E458ESF4)](https://codecov.io/gh/Nexucis/lamenv)
[![Godoc](https://godoc.org/github.com/nexucis/lamenv?status.svg)](https://pkg.go.dev/github.com/nexucis/lamenv)

Lamenv is a Go library that proposes a way to decode and encode the environment variable to/from a Golang struct.

## Installation

Standard `go get`:

```bash
$ go get github.com/nexucis/lamenv
```

## Purpose

There are plenty of library available to unmarshall a configuration using the environment. But, so far I didn't find one
that is able to unmarshall an array of struct or a map of struct. Also, each times other library required to add new
tags in the struct which bothered me.

I have written this library to be able to use it after spending times to support a yaml configuration. And then, you
think it could be interesting to get the config using the environment, but you don't want to spend time again to make it
work. For example, when moving to the cloud you forget a bit that you are using some password in your config, and an
easy way to get it is to use a secret that you can read in an environment variable.

And here is the solution ;). `lamenv`. Get it, use it, forget it.

More documentation is available in the [golang doc](https://pkg.go.dev/github.com/nexucis/lamenv) to know how to use it.

### Tips

To be able to unmarshall an array, lamenv is looking to the number of the index in the environment variable.

For example, you have the following struct:

```golang
type myConfig struct{
Test []string `yaml:"test"`
}
```

The environment variable should look like this if you want to map it to this struct:

```bash
MY_PREFIX_TEST_0="first value"
MY_PREFIX_TEST_1="another value"
```

This logic is used as well for an array of struct

```golang
type SliceOfStruct struct {
    A string `yaml:"a"`
}
type myConfig struct {
    Sli []SliceOfStruct `yaml:"slice"`
}
```

The environment variable should look like this if you want to map it to this struct:

```bash
MY_PREFIX_SLICE_0_A="first value"
MY_PREFIX_SLICE_1_A="another value"
```
