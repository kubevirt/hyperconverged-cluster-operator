# Deep Copy and Patch Library for Go

`deep` is a powerful, reflection-based library for creating deep copies, calculating differences (diffs), and patching complex Go data structures. It supports cyclic references, unexported fields, and custom type-specific behaviors.

## Features

*   **Deep Copy**: Recursively copies structs, maps, slices, arrays, pointers, and interfaces.
*   **Deep Diff**: Calculates the difference between two objects, producing a `Patch`.
*   **Patch Application**: Applies patches to objects to transform them from state A to state B.
*   **Patch Reversal**: Generates a reverse patch to undo changes (`Apply(Reverse(patch))`).
*   **Conditional Patching**: Apply patches only if specific logical conditions are met (`ApplyChecked`, `WithCondition`).
*   **Manual Patch Builder**: Construct valid patches manually using a fluent API with on-the-fly type validation.
*   **Unexported Fields**: Handles unexported struct fields transparently.
*   **Cycle Detection**: Correctly handles circular references in both Copy and Diff operations.

## Installation

```bash
go get github.com/brunoga/deep
```

## Usage

### Deep Copy

```go
import "github.com/brunoga/deep"

type Config struct {
    Name    string
    Version int
    Meta    map[string]any
}

src := Config{Name: "App", Version: 1, Meta: map[string]any{"env": "prod"}}
dst, err := deep.Copy(src)
if err != nil {
    panic(err)
}
```

### Deep Diff and Patch

Calculate the difference between two objects and apply it.

```go
oldConf := Config{Name: "App", Version: 1}
newConf := Config{Name: "App", Version: 2}

// Calculate Diff
patch := deep.Diff(oldConf, newConf)

// Check if there are changes
if patch != nil {
    fmt.Println("Changes found:", patch) 
    // Output: Struct{ Version: 1 -> 2 }

    // Apply to a target (must be a pointer)
    target := oldConf
    patch.Apply(&target)
    // target.Version is now 2
}
```

### Conditional Patching

You can attach conditions to a patch or check strict consistency before applying.

```go
// 1. Strict Application
// Checks that the target's current values match the 'old' values recorded in the patch.
err := patch.ApplyChecked(&target)
if err != nil {
    // Fails if target state has diverged from the original 'oldConf'
    log.Fatal("Conflict detected:", err)
}

// 2. Custom Logic Conditions
// Create a condition: Apply only if "Version" is greater than 0
cond, _ := deep.ParseCondition[Config]("Version > 0")
patchWithCond := patch.WithCondition(cond)

err = patchWithCond.ApplyChecked(&target)
```

**Supported Condition Syntax:**
*   **Comparisons**: `==`, `!=`, `>`, `<`, `>=`, `<=`
*   **Logic**: `AND`, `OR`, `NOT`, `(...)`
*   **Paths**: `Field`, `Field.SubField`, `Slice[0]`, `Map.Key`

### Patch Serialization

Patches can be serialized to JSON or Gob format for storage or transmission over the network.

#### JSON Serialization

```go
// Marshal
data, err := json.Marshal(patch)

// Unmarshal
newPatch := deep.NewPatch[Config]()
err = json.Unmarshal(data, newPatch)
```

#### Gob Serialization

When using Gob, you must register the `Patch` implementation for your type.

```go
// Register type once (e.g. in init())
deep.Register[Config]()

// Marshal
var buf bytes.Buffer
err := gob.NewEncoder(&buf).Encode(&patch)

// Unmarshal
newPatch := deep.NewPatch[Config]()
err = gob.NewDecoder(&buf).Decode(&newPatch)
```

### Manual Patch Builder

Construct patches programmatically without having two objects to compare.

```go
builder := deep.NewBuilder[Config]()
root := builder.Root()

// Set a field
root.Field("Name").Set("OldName", "NewName")

// Add to a map (if Config had a map)
// root.Field("Meta").MapKey("retry").Set(nil, 3)

patch, err := builder.Build()
if err == nil {
    patch.Apply(&myConfig)
}
```

### Reversing a Patch

Undo changes by creating a reverse patch.

```go
patch := deep.Diff(stateA, stateB)

// Apply forward
patch.Apply(&stateA) // stateA matches stateB

// Reverse
reversePatch := patch.Reverse()
reversePatch.Apply(&stateA) // stateA is back to original
```

## Advanced

### Custom Copier

Types can implement the `Copier[T]` interface to define custom copy behavior.

```go
type SecureToken string

func (t SecureToken) Copy() (SecureToken, error) {
    return "", nil // Don't copy tokens
}
```

### Unexported Fields

The library uses `unsafe` operations to read and write unexported fields. This is essential for true deep copying and patching of opaque structs but relies on internal runtime structures.

## License

Apache 2.0
