package dirtest

import (
	"io/fs"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDirFS(T *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(T, "DirFS Suite")
}

var _ = Describe("test DirFS", func() {
	It("should create an empty FileSystem", func() {
		dirFS := New()

		count := 0
		err := fs.WalkDir(dirFS, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d != nil {
				count++
			}

			return nil
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(count).To(Equal(1))
	})

	It("should create a directory", func() {
		dirFS := New(
			WithDir("myDir"),
		)

		count := 0
		err := fs.WalkDir(dirFS, "myDir", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d != nil {
				count++
			}

			return nil
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(count).To(Equal(1))

		info, err := fs.Stat(dirFS, "myDir")
		Expect(err).ToNot(HaveOccurred())
		Expect(info.IsDir()).To(BeTrue())
	})

	It("should create a nested directory", func() {
		dirFS := New(
			WithDir("myDir/nestedDir"),
		)

		count := 0
		err := fs.WalkDir(dirFS, "myDir", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d != nil {
				count++
			}

			return nil
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(count).To(Equal(2))

		info, err := fs.Stat(dirFS, "myDir")
		Expect(err).ToNot(HaveOccurred())
		Expect(info.IsDir()).To(BeTrue())

		info, err = fs.Stat(dirFS, "myDir/nestedDir")
		Expect(err).ToNot(HaveOccurred())
		Expect(info.IsDir()).To(BeTrue())
	})

	It("should create a file", func() {
		const fileName = "myFile.txt"
		content := []byte("hello world")

		dirFS := New(
			WithFile(fileName, content),
		)

		count := 0
		err := fs.WalkDir(dirFS, fileName, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d != nil {
				count++
			}

			return nil
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(count).To(Equal(1))

		info, err := fs.Stat(dirFS, fileName)
		Expect(err).ToNot(HaveOccurred())
		Expect(info.IsDir()).To(BeFalse())
		Expect(info.Size()).To(BeEquivalentTo(len(content)))

		data, err := fs.ReadFile(dirFS, fileName)
		Expect(err).ToNot(HaveOccurred())
		Expect(data).To(Equal(content))
	})

	It("should create a nested file", func() {
		const fileName = "a/b/c/myFile.txt"
		content := []byte("hello world")

		dirFS := New(
			WithFile(fileName, content),
		)

		count := 0
		err := fs.WalkDir(dirFS, "a", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d != nil {
				count++
			}

			return nil
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(count).To(Equal(4))

		info, err := fs.Stat(dirFS, fileName)
		Expect(err).ToNot(HaveOccurred())
		Expect(info.IsDir()).To(BeFalse())
		Expect(info.Size()).To(BeEquivalentTo(len(content)))

		data, err := fs.ReadFile(dirFS, fileName)
		Expect(err).ToNot(HaveOccurred())
		Expect(data).To(Equal(content))
	})

	It("should create a file in a directory", func() {
		const (
			dirName  = "dir"
			fileName = "dir/myFile.txt"
		)
		content := []byte("hello world")

		dirFS := New(
			WithDir(dirName),
			WithFile(fileName, content),
		)

		count := 0
		err := fs.WalkDir(dirFS, "dir", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d != nil {
				count++
			}

			return nil
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(count).To(Equal(2))

		info, err := fs.Stat(dirFS, fileName)
		Expect(err).ToNot(HaveOccurred())
		Expect(info.IsDir()).To(BeFalse())
		Expect(info.Size()).To(BeEquivalentTo(len(content)))

		data, err := fs.ReadFile(dirFS, fileName)
		Expect(err).ToNot(HaveOccurred())
		Expect(data).To(Equal(content))
	})

	It("should create multiple files in multiple directories", func() {

		files := map[string][]byte{
			"dir1/file1.txt":        []byte("dir1 file1 content"),
			"dir1/file2.txt":        []byte("dir1 file2 content"),
			"dir2/file1.txt":        []byte("dir2 file1 content"),
			"dir2/file2.txt":        []byte("dir2 file2 content"),
			"dir1/nested/file1.txt": []byte("dir1/nested file1 content"),
			"dir1/nested/file2.txt": []byte("dir1/nested file2 content"),
		}

		options := make([]Option, 0, len(files))
		for name, content := range files {
			options = append(options, WithFile(name, content))
		}

		dirFS := New(options...)

		count := 0
		err := fs.WalkDir(dirFS, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d != nil {
				count++
			}

			return nil
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(count).To(Equal(10))

		for fileName, content := range files {
			info, err := fs.Stat(dirFS, fileName)
			Expect(err).ToNot(HaveOccurred())
			Expect(info.IsDir()).To(BeFalse())
			Expect(info.Size()).To(BeEquivalentTo(len(content)))

			data, err := fs.ReadFile(dirFS, fileName)
			Expect(err).ToNot(HaveOccurred())
			Expect(data).To(Equal(content))
		}
	})
})
