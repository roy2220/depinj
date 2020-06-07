# depinj

[![GoDoc](https://godoc.org/github.com/roy2220/depinj?status.svg)](https://godoc.org/github.com/roy2220/depinj) [![Build Status](https://travis-ci.com/roy2220/depinj.svg?branch=master)](https://travis-ci.com/roy2220/depinj) [![Coverage Status](https://codecov.io/gh/roy2220/depinj/branch/master/graph/badge.svg)](https://codecov.io/gh/roy2220/depinj)

Dependency injection for Go

## Requirements

- Go 1.13

## Tutorial

1. [Import/Export by ref ID](#1-importexport-by-ref-id)
2. [Filter](#2-filter)
3. [Ref link](#3-ref-link)
4. [Import/Export by field type](#4-importexport-by-field-type)

### 1. Import/Export by ref ID

```go
package main

import (
        "context"
        "fmt"

        "github.com/roy2220/depinj"
)

func main() {
        podPool := depinj.PodPool{}
        podPool.MustAddPod(&StrangerA{})
        podPool.MustAddPod(&StrangerB{})
        podPool.MustSetUp(context.Background())
        podPool.TearDown()
        // Output: Hi!
}

type StrangerA struct {
        depinj.DummyPod // default implementation of depinj.Pod

        Greeting string `export:"the_greeting"` // export by ref id `the_greeting`
}

func (s *StrangerA) SetUp(context.Context) error {
        s.Greeting = "Hi!" // set the greeting
        return nil
}

type StrangerB struct {
        depinj.DummyPod // default implementation of depinj.Pod

        Greeting string `import:"the_greeting"` // import by ref id `the_greeting`
}

func (s *StrangerB) SetUp(context.Context) error {
        // fields of the import entries have been initialized, s.Greeting == "Hi!"
        fmt.Println(s.Greeting) // get the greeting
        return nil
}
```

### 2. Filter

```go
package main

import (
        "context"
        "fmt"

        "github.com/roy2220/depinj"
)

func main() {
        podPool := depinj.PodPool{}
        podPool.MustAddPod(&StrangerA{})
        podPool.MustAddPod(&StrangerB{})
        podPool.MustAddPod(&Hijacker{})
        podPool.MustSetUp(context.Background())
        podPool.TearDown()
        // Output: Hi! Jack!
}

type StrangerA struct {
        depinj.DummyPod // default implementation of depinj.Pod

        Greeting string `export:"the_greeting"` // export by ref id `the_greeting`
}

func (s *StrangerA) SetUp(context.Context) error {
        s.Greeting = "Hi!" // set the greeting
        return nil
}

type StrangerB struct {
        depinj.DummyPod // default implementation of depinj.Pod

        Greeting string `import:"the_greeting"` // import by ref id `the_greeting`
}

func (s *StrangerB) SetUp(context.Context) error {
        // fields of the import entries have been initialized and filtered, s.Greeting == "Hi! Jack!"
        fmt.Println(s.Greeting) // get the greeting
        return nil
}

type Hijacker struct {
        depinj.DummyPod // default implementation of depinj.Pod

        Greeting *string `filter:"the_greeting,ModifyGreeting,0"`
        // filter by ref id `the_greeting`, method `ModifyGreeting` and priority `0`
        //
        // The filter method is called after the pod has been set up (SetUp method),
        // it's safe to access the fields of the import or export entries in the pod,
        // which have been initialized, within the filter method.
        //
        // The higher priority value, the earlier call to the filter method, it's
        // useful if there are multiple filter entries for one export entry.
}

func (h *Hijacker) ModifyGreeting(context.Context) error {
        // modify the greeting after it has been exported and before it has been imported
        *h.Greeting += " Jack!"
        return nil
}
```

### 3. Ref link

```go
package main

import (
        "context"
        "fmt"

        "github.com/roy2220/depinj"
)

func main() {
        podPool := depinj.PodPool{}
        podPool.MustAddPod(&StrangerA{})
        podPool.MustAddPod(&StrangerB{})
        podPool.MustSetUp(context.Background())
        podPool.TearDown()
        // Output: Hello!
}

type StrangerA struct {
        depinj.DummyPod // default implementation of depinj.Pod

        Greeting1 string `export:"the_greeting_1"` // export by ref id `the_greeting_1`
        Greeting2 string `export:"the_greeting_2"` // export by ref id `the_greeting_2`
}

func (s *StrangerA) SetUp(context.Context) error {
        s.Greeting1 = "Hi!"    // set the greeting 1
        s.Greeting2 = "Hello!" // set the greeting 2
        return nil
}

type StrangerB struct {
        depinj.DummyPod // default implementation of depinj.Pod

        Greeting string `import:"@guess_what"` // import by ref link `@guess_what`
}

func (s *StrangerB) ResolveRefLink(refLink string) (string, bool) {
        // refLink == "@guess_what"
        return "the_greeting_2", true // resolve ref link `@guess_what` into ref id `the_greeting_2`
}

func (s *StrangerB) SetUp(context.Context) error {
        // fields of the import entries have been initialized, s.Greeting == "Hello!"
        fmt.Println(s.Greeting) // get the greeting
        return nil
}
```

### 4. Import/Export by field type

```go
package main

import (
        "context"
        "errors"
        "fmt"

        "github.com/roy2220/depinj"
)

func main() {
        podPool := depinj.PodPool{}
        podPool.MustAddPod(&Foo{})
        podPool.MustAddPod(&Bar{})
        podPool.MustSetUp(context.Background())
        podPool.TearDown()
        // Output: unknown error
}

type Foo struct {
        depinj.DummyPod // default implementation of depinj.Pod

        Err error `export:""` // ref id omitted, export by field type `error`
}

var ErrUnknown = errors.New("unknown error")

func (f *Foo) SetUp(context.Context) error {
        f.Err = ErrUnknown // set the error
        return nil
}

type Bar struct {
        depinj.DummyPod // default implementation of depinj.Pod

        Err error `import:""` // ref id omitted, import by field type `error`
}

func (b *Bar) SetUp(context.Context) error {
        // fields of the import entries have been initialized, b.Err == ErrUnknown
        fmt.Println(b.Err) // get the error
        return nil
}
```
