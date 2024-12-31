package stream

import (
	"github.com/onsi/gomega"
	"maps"
	"slices"
	"strconv"
	"testing"
)

func TestTransform(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	var s []int
	g.Expect(slices.Collect(Transform(slices.Values(s), strconv.Itoa))).To(gomega.BeNil())

	s = []int{1, 2, 3}

	res := slices.Collect(Transform(slices.Values(s), strconv.Itoa))
	g.Expect(res).To(gomega.HaveLen(3))
	g.Expect(res).To(gomega.ContainElements("1", "2", "3"))
}

func TestTransform2(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	var s []int
	trans2Func := func(i int) (int, string) {
		return i, strconv.Itoa(i)
	}

	g.Expect(maps.Collect(Transform2(slices.Values(s), trans2Func))).To(gomega.BeEmpty())

	s = []int{1, 2, 3}

	res := maps.Collect(Transform2(slices.Values(s), trans2Func))
	g.Expect(res).To(gomega.HaveLen(3))
	g.Expect(res).To(gomega.HaveKeyWithValue(1, "1"))
	g.Expect(res).To(gomega.HaveKeyWithValue(2, "2"))
	g.Expect(res).To(gomega.HaveKeyWithValue(3, "3"))
}

func TestTransform22(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	var s []int
	trans22Func := func(i, v int) (int, string) {
		return i * i, strconv.Itoa(v)
	}

	g.Expect(maps.Collect(Transform22(slices.All(s), trans22Func))).To(gomega.BeEmpty())

	s = []int{11, 22, 33, 44}

	res := maps.Collect(Transform22(slices.All(s), trans22Func))
	g.Expect(res).To(gomega.HaveLen(4))
	g.Expect(res).To(gomega.HaveKeyWithValue(0, "11"))
	g.Expect(res).To(gomega.HaveKeyWithValue(1, "22"))
	g.Expect(res).To(gomega.HaveKeyWithValue(4, "33"))
	g.Expect(res).To(gomega.HaveKeyWithValue(9, "44"))

	res2 := slices.Collect(Iter2Values(Transform22(slices.All(s), trans22Func)))
	g.Expect(res2).To(gomega.HaveLen(4))
	g.Expect(res2).To(gomega.ContainElements("11", "22", "33", "44"))
}

func TestFilter(t *testing.T) {
	g := gomega.NewGomegaWithT(t)
	var s []int

	isEven := func(i int) bool { return i&1 == 0 }
	g.Expect(slices.Collect(Filter(slices.Values(s), isEven))).To(gomega.BeNil())
	s = []int{1, 2, 3, 4, 5, 6, 7, 8, 9}
	res := slices.Collect(Filter(slices.Values(s), isEven))
	g.Expect(res).To(gomega.HaveLen(4))
	g.Expect(res).To(gomega.ContainElements(2, 4, 6, 8))
}
