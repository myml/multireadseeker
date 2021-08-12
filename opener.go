package multireadseeker

import "io"

// Opener 文件的抽象封装
type Opener interface {
	Open() (io.ReadSeekCloser, error)
}

// OpenFunc opener的简单函数实现
// 例子 Server(OpenFunc(func() (io.ReadSeekCloser, error) { return os.Open(filename) }), nil)
// 更多实现见tarhttp_test.go
type OpenFunc func() (io.ReadSeekCloser, error)

// Open 实现Opener接口
func (f OpenFunc) Open() (io.ReadSeekCloser, error) {
	return f()
}

// openerSize 用于获取Opener的大小，通过Seek的方式
func openerSize(o Opener) (int64, error) {
	f, err := o.Open()
	if err != nil {
		return 0, err
	}
	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}
	err = f.Close()
	if err != nil {
		return 0, err
	}
	return size, nil
}
