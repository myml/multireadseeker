package multireadseeker

import (
	"errors"
	"io"
)

type opener struct {
	Opener
	size int64
}

// NewMultiOpener 合并多个Opener接口为一个，将文件切片虚拟的合并
func NewMultiOpener(os ...Opener) (Opener, error) {
	mo := multiOpener{}
	// 获取所有文件的大小，并进行记录
	for i := range os {
		size, err := openerSize(os[i])
		if err != nil {
			return nil, err
		}
		mo.size += size
		mo.childen = append(mo.childen, opener{Opener: os[i], size: size})
	}
	return &mo, nil
}

type multiOpener struct {
	childen []opener
	size    int64
}

func (mo *multiOpener) Open() (io.ReadSeekCloser, error) {
	// 预先打开第一个文件分片
	var first io.ReadSeekCloser
	if len(mo.childen) > 0 {
		var err error
		first, err = mo.childen[0].Open()
		if err != nil {
			return nil, err
		}
	}
	return &multiOpenerReader{childen: mo.childen, curren: first, size: mo.size}, nil
}

type multiOpenerReader struct {
	childen []opener
	curren  io.ReadSeekCloser
	index   int
	size    int64
	offset  int64
}

// 流读取，如果分片读取到末尾，则自动切换到下一个分片
func (mor *multiOpenerReader) Read(p []byte) (int, error) {
	if mor.index >= len(mor.childen) {
		return 0, io.EOF
	}
	// 从当前的child读取数据
	n, err := mor.curren.Read(p)
	// 记录流的偏移
	mor.offset += int64(n)
	if err != nil {
		if err != io.EOF {
			return n, err
		}
		// 如果当前child读取到末尾
		// 并且是最后一个child则返回EOF，否则切换到下一个child
		if mor.index+1 >= len(mor.childen) {
			return n, err
		}
		// 关闭当前分片
		err = mor.curren.Close()
		if err != nil {
			return 0, err
		}
		// 切换到下一个分片
		mor.index++
		mor.curren, err = mor.childen[mor.index].Open()
		if err != nil {
			return 0, err
		}
	}
	return n, nil
}

// 流跳转，计算出跳转的位置，将位置所在的切片设置为当前切片
func (mor *multiOpenerReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		mor.offset = offset
	case io.SeekCurrent:
		mor.offset += offset
	case io.SeekEnd:
		mor.offset = mor.size + offset
	default:
		return 0, errors.New("Seek: invalid whence")
	}
	// 修改切片索引超出范围，下面循环定位正确位置
	mor.index = len(mor.childen)
	// 关闭当前切片
	err := mor.curren.Close()
	if err != nil {
		return 0, err
	}
	var count int64
	for i := range mor.childen {
		// 当跳转位置位于某个切片中
		if mor.offset < count+mor.childen[i].size {
			curren, err := mor.childen[i].Open()
			if err != nil {
				return 0, err
			}
			mor.curren, mor.index = curren, i
			// 切片也需要跳转到相对位置
			_, err = mor.curren.Seek(mor.offset-count, io.SeekStart)
			if err != nil {
				return 0, err
			}
			break
		}
		count += mor.childen[i].size
	}
	return mor.offset, nil
}

func (mor *multiOpenerReader) Close() error {
	if mor.curren == nil {
		return nil
	}
	return mor.curren.Close()
}
