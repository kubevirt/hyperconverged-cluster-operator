package lamenv

import (
	"fmt"
	"reflect"
	"strings"
)

type nodeKind uint

const (
	root nodeKind = iota
	node
	nodeSquashed
	leaf
	// deadLeaf will design a leaf which is not the end of a tree but more a node where we cannot determinate what is coming next
	deadLeaf
)

// ring will represent the struct hold by reflect.Type
type ring struct {
	kind     nodeKind
	value    string
	children []*ring
}

func newRing(t reflect.Type, tag []string) *ring {
	root := &ring{
		kind: root,
	}
	root.buildRing(t, tag)
	return root
}

func (r *ring) buildRing(t reflect.Type, tag []string) {
	switch t.Kind() {
	case reflect.Ptr:
		r.buildRing(t.Elem(), tag)
	case reflect.Slice,
		reflect.Array:
		if len(r.value) > 0 {
			r.value = r.value + "_0"
		} else {
			r.value = "0"
		}
		r.buildRing(t.Elem(), tag)
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if len(t.Field(i).PkgPath) > 0 {
				// the field is not exported, so no need to look at it as we won't be able to set it in a later stage
				continue
			}
			field := t.Field(i)
			var fieldName string
			tags, ok := lookupTag(field.Tag, tag)
			kind := node
			if ok {
				fieldName = tags[0]
				tags = tags[1:]
				if fieldName == "-" {
					continue
				}
				if containStr(tags, squash) || containStr(tags, inline) {
					// in this case it just means the next node won't provide any additional value
					fieldName = ""
					kind = nodeSquashed
				}
			} else {
				fieldName = field.Name
			}
			node := &ring{
				kind:  kind,
				value: strings.ToUpper(fieldName),
			}
			node.buildRing(field.Type, tag)
			r.children = append(r.children, node)
		}
	case reflect.Map,
		reflect.Interface:
		r.kind = deadLeaf
	default:
		r.kind = leaf
	}
}

type possiblePrefix struct {
	// value is the actual value of the prefix
	value string
	// startPos is the starting position of the next token that was matching the value used to calculate the prefix
	startPos int
	// endPos is the
	endPos int
}

func findPrefixes(parts []string, pos int, value string) []possiblePrefix {
	var result []possiblePrefix
	for i := pos; i < len(parts); i++ {
		matched, p := consumePart(parts, i, value)
		if matched && i > 0 {
			result = append(result, possiblePrefix{
				value:    strings.Join(parts[:i], "_"),
				startPos: i,
				endPos:   p,
			})
		}
	}
	return result
}

func consumePart(parts []string, pos int, value string) (bool, int) {
	aggregatedValue := parts[pos]
	i := pos + 1
	for i < len(parts) && aggregatedValue != value {
		aggregatedValue = fmt.Sprintf("%s_%s", aggregatedValue, parts[i])
		i++
	}
	if aggregatedValue != value {
		return false, 0
	}
	return true, i - 1
}

// guessPrefix is a way to determinate what is the missing prefix key that would complete the parts in order to have a complete ring
func guessPrefix(parts []string, r *ring) (string, error) {
	if r.kind != root && r.kind != leaf {
		return "", fmt.Errorf("unable to determinate the number of paths, ring is not the root or a leaf")
	}

	if r.kind == leaf {
		if len(r.value) == 0 {
			return strings.Join(parts, "_"), nil
		} else {
			prefixes := findPrefixes(parts, 0, r.value)
			if len(prefixes) == 0 {
				return "", nil
			}
			for _, prefix := range prefixes {
				if prefix.startPos+1 == len(parts) {
					return prefix.value, nil
				}
			}
			return "", fmt.Errorf("too many possible prefix for the leaf with the value %s", r.value)
		}
	}
	// here we have to make a bfs (breadth-first search) into the tree that would stop once it doesn't find any child that has an empty value.
	return bfs(parts, r)
}

func bfs(parts []string, r *ring) (string, error) {
	nodes := []*ring{r}
	var result string
	var paths uint64 = 0
	for len(nodes) > 0 {
		current := nodes[0]
		// Remove the first element of the file, since it is currently treated
		nodes = nodes[1:]
		if len(current.value) == 0 {
			// in that case we are putting all its children to be treated.
			nodes = append(nodes, current.children...)
			// and then we move to the next ring since the current node won't help to guess which prefix do we need
			continue
		}
		// treatment of the current node
		prefixes := findPrefixes(parts, 0, current.value)
		switch current.kind {
		case leaf:
			for _, prefix := range prefixes {
				// since it's a leaf, that means there would be nothing after this node. So the parts must be totally consumed by the prefix + the value of the current ring.
				// Otherwise it's not a correct prefix.
				if prefix.endPos+1 == len(parts) {
					paths++
					if paths > 1 {
						return "", fmt.Errorf("too many possibilities available when choosing the key '%s'", prefix.value)
					}
					if paths == 1 {
						result = prefix.value
					}
				}
			}
		case deadLeaf:
			for _, prefix := range prefixes {
				// since it's a deadLeaf, that means there would be something after this node. So the parts cannot be totally consumed by the prefix + the value of the current ring.
				// Otherwise it's not a correct prefix.
				if prefix.endPos+1 < len(parts) {
					paths++
					if paths > 1 {
						return "", fmt.Errorf("too many possibilities available when choosing the key '%s'", prefix.value)
					}
					if paths == 1 {
						result = prefix.value
					}
				}
			}
		default:
			// Test every possible path for each prefix find.
			// And hope there would one or less possible path at the end.
			for _, prefix := range prefixes {
				for _, child := range current.children {
					pathPossibility(parts, prefix.endPos+1, child, &paths)
					if paths > 1 {
						return "", fmt.Errorf("too many possibilities available when choosing the key '%s'", prefix.value)
					}
					if paths == 1 {
						result = prefix.value
					}
				}
			}
		}
	}
	return result, nil
}

// pathPossibility will return the number of possible path depending of the available tree and the given parts
func pathPossibility(parts []string, pos int, r *ring, result *uint64) {
	if len(r.value) > 0 {
		if pos >= len(parts) {
			// we are outside of the given parts, so it means there is no path that is matching the given parts
			return
		}
		// here we have to determinate if value is a concatenation of multiple value of parts
		// If it's not the case, then the path doesn't exist
		matched, p := consumePart(parts, pos, r.value)
		if !matched {
			// as the value doesn't match any aggregation, then the path doesn't exist
			return
		}
		pos = p
		switch r.kind {
		case deadLeaf:
			// here we can only say the path exists if we didn't consume all the parts
			// If you have a map for example, then that means it requires at least one key to set, so at least one more value in the parts.
			// So if the position +1 exceed the size of the parts, then there is no remaining key for the value of the map.
			if pos+1 < len(parts) {
				*result = *result + 1
			}
		case leaf:
			// if it is a leaf, then the path exists only if the parts are totally consumed
			if pos+1 == len(parts) {
				*result = *result + 1
			}
		case node,
			nodeSquashed:
			// if it is a node, then we just have to increase the position and restart the calculation for each child
			for _, child := range r.children {
				pathPossibility(parts, pos+1, child, result)
			}
		}
	} else {
		switch r.kind {
		case deadLeaf:
			// so the value is empty and it is a deadLeaf. Which means it doesn't matter what is the remaining parts, the path exists.
			*result = *result + 1
		case leaf:
			// here the path exists only if we reached the end of the parts, since it means that the value would come from the parent ring
			if pos == len(parts) {
				*result = *result + 1
			}
		case nodeSquashed:
			// here we just have to ignore the current ring and move to the next one without increasing the position
			for _, child := range r.children {
				pathPossibility(parts, pos, child, result)
			}
		case node:
			// at this point, this case cannot exist, so it's better to say there is no path that would match this possibility
			return
		}
	}
}
