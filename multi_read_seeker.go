package multireadseeker

import (
	"errors"
	"io"
)

var _ io.ReadSeekCloser = new(multiReadSeeker)

type multiReadSeeker struct {
	childen []*multiReadSeekerChild
	index   int
	size    int64
	offset  int64
}

type multiReadSeekerChild struct {
	size int64
	io.ReadSeekCloser
}

func (mrs *multiReadSeeker) Read(p []byte) (int, error) {
	if mrs.index >= len(mrs.childen) {
		return 0, io.EOF
	}
	// 从当前的child读取数据
	n, err := mrs.childen[mrs.index].Read(p)
	mrs.offset += int64(n)
	if err != nil {
		if err != io.EOF {
			return n, err
		}
		// 如果当前child读取到末尾
		// 并且是最后一个child则返回EOF，否则切换到下一个child
		if mrs.index+1 >= len(mrs.childen) {
			return n, err
		}
		mrs.index++
		_, err = mrs.childen[mrs.index].Seek(0, io.SeekStart)
		if err != nil {
			return 0, err
		}
	}
	return n, nil
}

func (mrs *multiReadSeeker) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		mrs.offset = offset
	case io.SeekCurrent:
		mrs.offset += offset
	case io.SeekEnd:
		mrs.offset = mrs.size + offset
	default:
		return 0, errors.New("Seek: invalid whence")
	}
	// 先设置切片索引超出范围，下面循环定位正确位置
	mrs.index = len(mrs.childen)
	var count int64
	for i := range mrs.childen {
		// 找到位置所在切片
		if mrs.offset < count+mrs.childen[i].size {
			mrs.index = i
			// 切片自身也需要做相对跳转
			mrs.childen[i].Seek(mrs.offset-count, io.SeekStart)
			break
		}
		count += mrs.childen[i].size
	}
	return mrs.offset, nil
}

func (mrs *multiReadSeeker) Close() error {
	// 关闭所有切片
	for i := range mrs.childen {
		err := mrs.childen[i].Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// NewMultiReadSeeker 用于将多个流合并为一个流，可用于文件分片合并
func NewMultiReadSeeker(rs ...io.ReadSeekCloser) (io.ReadSeekCloser, error) {
	mrs := multiReadSeeker{}
	for i := range rs {
		size, err := rs[i].Seek(0, io.SeekEnd)
		if err != nil {
			return nil, err
		}
		_, err = rs[i].Seek(0, io.SeekStart)
		if err != nil {
			return nil, err
		}
		mrs.size += size
		mrs.childen = append(mrs.childen, &multiReadSeekerChild{ReadSeekCloser: rs[i], size: size})
	}
	return &mrs, nil
}
