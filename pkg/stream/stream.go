package stream

import "iter"

// Filter creates an iterator that filters the elements of a sequence (iterator), by a given function filterFn
// that for each element of type V1, returns true to include the element in the result sequence, or false, to
// skip the element.
// The Filter function returns an iter.Seq sequence.
func Filter[T any](seq iter.Seq[T], filterFn func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range seq {
			if filterFn(v) {
				if !yield(v) {
					return
				}
			}
		}
	}
}

// Transform creates an iterator that modifies the elements of a sequence (iterator), by a given function transFn
// that for each element of type V1, returns a value V2.
// The Transform function returns an iter.Seq sequence.
func Transform[V1, V2 any](seq iter.Seq[V1], transFn func(V1) V2) iter.Seq[V2] {
	return func(yield func(V2) bool) {
		for v1 := range seq {
			if !yield(transFn(v1)) {
				return
			}
		}
	}
}

// Transform2 creates an iterator that modifies the elements of a sequence (iterator), by a given function transFn
// that for each element of type V1, returns a key K and a value V2.
// The Transform2 function returns an iter.Seq2 sequence.
func Transform2[K, V1, V2 any](seq iter.Seq[V1], transFn func(V1) (K, V2)) iter.Seq2[K, V2] {
	return func(yield func(K, V2) bool) {
		for v1 := range seq {
			if !yield(transFn(v1)) {
				return
			}
		}
	}
}

// Transform22 creates an iterator that modifies the elements of a sequence (iterator), by a given function transFn
// that for each pair of K1, V1, returns a pair of K2, V2.
// The Transform22 function returns an iter.Seq2 sequence.
func Transform22[K1, V1, K2, V2 any](seq iter.Seq2[K1, V1], transFn func(K1, V1) (K2, V2)) iter.Seq2[K2, V2] {
	return func(yield func(K2, V2) bool) {
		for k1, v1 := range seq {
			if !yield(transFn(k1, v1)) {
				return
			}
		}
	}
}

func Iter2Values[K, V any](seq2 iter.Seq2[K, V]) iter.Seq[V] {
	return func(yield func(V) bool) {
		for _, v := range seq2 {
			if !yield(v) {
				return
			}
		}
	}
}
