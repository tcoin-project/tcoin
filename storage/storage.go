package storage

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type recycle struct {
	height int
	id     SliceKeyType
}

type StorageEngine struct {
	config    StorageEngineConfig
	ss        map[SliceKeyType]*Slice
	fa        map[SliceKeyType]SliceKeyType
	son       map[SliceKeyType][]SliceKeyType
	data      map[SliceKeyType][]byte
	ldata     map[int][]byte
	root      SliceKeyType
	rq        []recycle
	fDataPosR *os.File
	fDataPosW *os.File
	fDataR    *os.File
	fDataW    *os.File
	rootMut   chan bool
	stop      chan bool
	stopped   chan bool
	flush     chan bool
	flushed   chan error
	ldMut     sync.Mutex
}

func NewStorageEngine(config StorageEngineConfig, initSlice *Slice, initKey SliceKeyType, initData []byte) (*StorageEngine, error) {
	if initSlice.height != 0 {
		return nil, errors.New("init slice must have height 0")
	}
	err := os.MkdirAll(filepath.Join(config.Path, "perm"), 0o755)
	if err != nil {
		return nil, fmt.Errorf("error when creating storage engine: %v", err)
	}
	err = os.MkdirAll(filepath.Join(config.Path, "temp"), 0o755)
	if err != nil {
		return nil, fmt.Errorf("error when creating storage engine: %v", err)
	}
	// todo: init, read file, fdata
	e := &StorageEngine{
		config:  config,
		ss:      make(map[SliceKeyType]*Slice),
		fa:      make(map[SliceKeyType]SliceKeyType),
		son:     make(map[SliceKeyType][]SliceKeyType),
		data:    make(map[SliceKeyType][]byte),
		ldata:   make(map[int][]byte),
		root:    SliceKeyType{},
		rq:      []recycle{},
		rootMut: make(chan bool, 1),
		stop:    make(chan bool, 1),
		stopped: make(chan bool, 1),
		flush:   make(chan bool, 1),
		flushed: make(chan error, 1),
		ldMut:   sync.Mutex{},
	}
	dataFn := filepath.Join(config.Path, "perm", "data")
	dataPosFn := filepath.Join(config.Path, "perm", "datapos")
	ssFn := filepath.Join(e.config.Path, "perm", "ss")
	if _, err := os.Stat(ssFn); errors.Is(err, os.ErrNotExist) {
		e.ss[initKey] = initSlice
		e.data[initKey] = initData
		e.root = initKey
		e.fDataW, err = os.OpenFile(dataFn, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("error when creating storage engine: %v", err)
		}
		e.fDataPosW, err = os.OpenFile(dataPosFn, os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("error when creating storage engine: %v", err)
		}
		e.fDataR, err = os.Open(dataFn)
		if err != nil {
			return nil, fmt.Errorf("error when creating storage engine: %v", err)
		}
		e.fDataPosR, err = os.Open(dataPosFn)
		if err != nil {
			return nil, fmt.Errorf("error when creating storage engine: %v", err)
		}
		e.rq = append(e.rq, recycle{
			height: -1,
			id:     SliceKeyType{},
		})
		e.ldata[0] = initData
		err = e.storeRoot()
		if err != nil {
			return nil, fmt.Errorf("error when creating storage engine: %v", err)
		}
	} else {
		e.fDataW, err = os.OpenFile(dataFn, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("error when loading storage engine: %v", err)
		}
		e.fDataPosW, err = os.OpenFile(dataPosFn, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("error when loading storage engine: %v", err)
		}
		e.fDataR, err = os.Open(dataFn)
		if err != nil {
			return nil, fmt.Errorf("error when loading storage engine: %v", err)
		}
		e.fDataPosR, err = os.Open(dataPosFn)
		if err != nil {
			return nil, fmt.Errorf("error when loading storage engine: %v", err)
		}
		f, err := os.Open(ssFn)
		if err != nil {
			return nil, fmt.Errorf("error when loading storage engine: %v", err)
		}
		st := EmptySlice()
		err = st.LoadFile(bufio.NewReader(f))
		if err != nil {
			return nil, fmt.Errorf("error when loading storage engine: %v", err)
		}
		key, err := e.ReadKey(st.height)
		if err != nil {
			return nil, fmt.Errorf("error when loading storage engine: %v", err)
		}
		offset, length, err := e.ReadOffset(st.height)
		if err != nil {
			return nil, fmt.Errorf("error when loading storage engine: %v", err)
		}
		_, err = e.fDataPosW.Seek(int64((st.height+1)*SliceDataPosLen), io.SeekStart)
		if err != nil {
			return nil, fmt.Errorf("error when loading storage engine: %v", err)
		}
		_, err = e.fDataW.Seek(int64(offset+length), io.SeekStart)
		if err != nil {
			return nil, fmt.Errorf("error when loading storage engine: %v", err)
		}
		e.ss[key] = st
		e.root = key
	}
	e.loadSubtrees()
	e.rootMut <- true
	go e.StoreFinalizedSlices(e.root)
	return e, nil
}

type subtreeCandidate struct {
	key SliceKeyType
	s   *Slice
}

type subtreeCandidates []subtreeCandidate

func (s subtreeCandidates) Len() int {
	return len(s)
}

func (s subtreeCandidates) Less(i, j int) bool {
	return s[i].s.height < s[j].s.height
}

func (s subtreeCandidates) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (e *StorageEngine) loadSubtrees() {
	files, err := ioutil.ReadDir(filepath.Join(e.config.Path, "temp"))
	if err != nil {
		return
	}
	var keys []SliceKeyType
	for _, file := range files {
		if len(file.Name()) == SliceKeyLen*2 {
			bs, err := hex.DecodeString(file.Name())
			if err == nil && len(bs) == SliceKeyLen {
				var key SliceKeyType
				copy(key[:], bs)
				keys = append(keys, key)
			}
		}
	}
	ss := subtreeCandidates{}
	for _, key := range keys {
		f, err := os.Open(e.GetTempFileName(key))
		if err != nil {
			continue
		}
		s := EmptySlice()
		err = s.LoadFile(bufio.NewReader(f))
		if err != nil {
			f.Close()
			continue
		}
		f.Close()
		ss = append(ss, subtreeCandidate{
			key: key,
			s:   s,
		})
	}
	sort.Sort(ss)
	for _, sc := range ss {
		d, err := os.ReadFile(e.GetTempFileName(sc.key) + ".b")
		if err != nil || len(d) < SliceKeyLen {
			continue
		}
		var fa SliceKeyType
		copy(fa[:], d[:SliceKeyLen])
		if fas, ok := e.ss[fa]; ok {
			sc.s.base = fas
			e.ss[sc.key] = sc.s
			e.fa[sc.key] = fa
			e.data[sc.key] = d[SliceKeyLen:]
			u, ok := e.son[fa]
			if ok {
				e.son[fa] = append(u, sc.key)
			} else {
				e.son[fa] = []SliceKeyType{sc.key}
			}
		}
	}
}

func (e *StorageEngine) GetTempFileName(k SliceKeyType) string {
	return filepath.Join(e.config.Path, "temp", hex.EncodeToString(k[:]))
}

// freeze a slice (no further write operations)
func (e *StorageEngine) AddFreezedSlice(s *Slice, k SliceKeyType, f SliceKeyType, data []byte) error {
	if !s.freezed {
		return errors.New("not freezed yet")
	}
	if _, ok := e.ss[k]; ok {
		return errors.New("key already exists in storage engine")
	}
	e.ss[k] = s
	e.fa[k] = f
	e.data[k] = data
	u, ok := e.son[f]
	if ok {
		e.son[f] = append(u, k)
	} else {
		e.son[f] = []SliceKeyType{k}
	}
	fl, err := os.OpenFile(e.GetTempFileName(k), os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("error when creating file %s: %v", e.GetTempFileName(k), err)
	}
	bf := bufio.NewWriter(fl)
	err = s.DumpFile(bf)
	if err != nil {
		fl.Close()
		return fmt.Errorf("error when writing file %s: %v", e.GetTempFileName(k), err)
	}
	err = bf.Flush()
	if err != nil {
		fl.Close()
		return fmt.Errorf("error when writing file %s: %v", e.GetTempFileName(k), err)
	}
	err = fl.Close()
	if err != nil {
		return fmt.Errorf("error when closing file %s: %v", e.GetTempFileName(k), err)
	}
	err = ioutil.WriteFile(e.GetTempFileName(k)+".b", append(f[:], data...), 0o755)
	if err != nil {
		return fmt.Errorf("error when writing file %s: %v", e.GetTempFileName(k)+".b", err)
	}
	err = e.FinalizeSlice(k)
	if err != nil {
		return fmt.Errorf("error when finalizing %s: %v", hex.EncodeToString(k[:]), err)
	}
	return nil
}

// finalize a slice (can't change it anymore)
func (e *StorageEngine) FinalizeSlice(k SliceKeyType) error {
	for i := 0; i < e.config.FinalizeDepth; i++ {
		var ok bool
		k, ok = e.fa[k]
		if !ok {
			return nil
		}
	}
	t := k
	for {
		fa, ok := e.fa[t]
		if !ok {
			break
		}
		sons := e.son[fa]
		for i := 0; i < len(sons); i++ {
			if sons[i] != t {
				e.discardSubtree(sons[i])
			}
		}
		e.son[fa] = []SliceKeyType{t}
		t = fa
	}
	select {
	case <-e.rootMut:
		e.mergeFa(k)
		e.rootMut <- true
	default:
	}
	return nil
}

func (e *StorageEngine) discardSubtree(k SliceKeyType) {
	sons, ok := e.son[k]
	if ok {
		for i := 0; i < len(sons); i++ {
			e.discardSubtree(sons[i])
		}
		delete(e.son, k)
	}
	os.Remove(e.GetTempFileName(k))
	os.Remove(e.GetTempFileName(k) + ".b")
	delete(e.ss, k)
	delete(e.fa, k)
	delete(e.data, k)
}

func (e *StorageEngine) mergeFa(k SliceKeyType) {
	fa, ok := e.fa[k]
	if !ok {
		return
	}
	e.mergeFa(fa)
	a := e.ss[fa]
	b := e.ss[k]
	for k, v := range b.st {
		a.st[k] = v
	}
	b.st = a.st
	b.base = nil
	delete(e.ss, fa)
	delete(e.fa, k)
	e.ldMut.Lock()
	e.ldata[a.height] = e.data[fa]
	e.ldMut.Unlock()
	delete(e.data, fa)
	delete(e.son, fa)
	e.rq = append(e.rq, recycle{
		height: a.height,
		id:     fa,
	})
	e.root = k
}

func (e *StorageEngine) ReadKey(height int) (SliceKeyType, error) {
	_, err := e.fDataPosR.Seek(int64(SliceDataPosLen*height+16), io.SeekStart)
	if err != nil {
		return SliceKeyType{}, fmt.Errorf("failed to read key of height %d: %v", height, err)
	}
	var res SliceKeyType
	_, err = io.ReadFull(e.fDataPosR, res[:])
	if err != nil {
		return SliceKeyType{}, fmt.Errorf("failed to read key of height %d: %v", height, err)
	}
	return res, nil
}

func (e *StorageEngine) ReadOffset(height int) (int, int, error) {
	_, err := e.fDataPosR.Seek(int64(SliceDataPosLen*height), io.SeekStart)
	if err != nil {
		return 0, 0, err
	}
	td := make([]byte, 16)
	_, err = io.ReadFull(e.fDataPosR, td)
	if err != nil {
		return 0, 0, err
	}
	a := int(binary.LittleEndian.Uint64(td[:8]))
	b := int(binary.LittleEndian.Uint64(td[8:]))
	return a, b, nil
}

func (e *StorageEngine) ReadData(height int, k SliceKeyType) ([]byte, error) {
	d, ok := e.data[k]
	if ok {
		return d, nil
	}
	e.ldMut.Lock()
	d, ok = e.ldata[height]
	e.ldMut.Unlock()
	if ok {
		return d, nil
	}
	npos, length, err := e.ReadOffset(height)
	if err != nil {
		return nil, fmt.Errorf("failed to read data at slice %s height %d: %v", hex.EncodeToString(k[:]), height, err)
	}
	_, err = e.fDataR.Seek(int64(npos), io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to read data at slice %s height %d: %v", hex.EncodeToString(k[:]), height, err)
	}
	res := make([]byte, length)
	_, err = io.ReadFull(e.fDataR, res)
	if err != nil {
		return nil, fmt.Errorf("failed to read data at slice %s height %d: %v", hex.EncodeToString(k[:]), height, err)
	}
	return res, nil
}

func (e *StorageEngine) storeRoot() (errr error) {
	dpos, _ := e.fDataW.Seek(0, io.SeekCurrent)
	ppos, _ := e.fDataPosW.Seek(0, io.SeekCurrent)
	defer func() {
		if errr != nil {
			e.fDataW.Seek(dpos, io.SeekStart)
			e.fDataPosW.Seek(ppos, io.SeekStart)
		}
	}()
	if len(e.rq) > 0 {
		if ppos != int64((e.rq[0].height+1)*SliceDataPosLen) {
			return errors.New("file seek mismatch")
		}
		td := make([]byte, SliceDataPosLen)
		curh := e.rq[0].height + 1
		for i := 1; i <= len(e.rq); i++ {
			var h int
			var ud []byte
			var id SliceKeyType
			if i < len(e.rq) {
				h = e.rq[i].height
				ud = e.ldata[h]
				id = e.rq[i].id
			} else {
				h = e.ss[e.root].height
				ud = e.data[e.root]
				id = e.root
			}
			if h != curh {
				return errors.New("recycle queue height mismatch")
			}
			curh++
			pos, _ := e.fDataW.Seek(0, io.SeekCurrent)
			binary.LittleEndian.PutUint64(td[:8], uint64(pos))
			binary.LittleEndian.PutUint64(td[8:16], uint64(len(ud)))
			copy(td[16:], id[:])
			_, err := e.fDataPosW.Write(td)
			if err != nil {
				return err
			}
			_, err = e.fDataW.Write(ud)
			if err != nil {
				return err
			}
		}
		err := e.fDataPosW.Sync()
		if err != nil {
			return err
		}
		err = e.fDataW.Sync()
		if err != nil {
			return err
		}
	}
	fl, err := os.Create(filepath.Join(e.config.Path, "perm", "ss.next"))
	if err != nil {
		return err
	}
	bf := bufio.NewWriter(fl)
	err = e.ss[e.root].DumpFile(bf)
	if err != nil {
		fl.Close()
		return err
	}
	err = bf.Flush()
	if err != nil {
		fl.Close()
		return err
	}
	err = fl.Close()
	if err != nil {
		return err
	}
	err = os.Rename(filepath.Join(e.config.Path, "perm", "ss.next"), filepath.Join(e.config.Path, "perm", "ss"))
	if err != nil {
		return err
	}
	e.ldMut.Lock()
	for i := 0; i < len(e.rq); i++ {
		delete(e.ldata, e.rq[i].height)
	}
	e.ldMut.Unlock()
	for i := 0; i < len(e.rq); i++ {
		os.Remove(e.GetTempFileName(e.rq[i].id))
		os.Remove(e.GetTempFileName(e.rq[i].id) + ".b")
	}
	e.rq = []recycle{}
	return nil
}

func (e *StorageEngine) StoreFinalizedSlices(lastRoot SliceKeyType) {
	sleepTime := time.Second * 5
	for {
		sleep := time.After(sleepTime)
		sleepTime = time.Second * 5
		select {
		case <-e.stop:
			e.stopped <- true
			return
		case <-e.flush:
			e.flushed <- e.storeRoot()
			continue
		case <-sleep:
		}
		select {
		case <-e.rootMut:
			if e.root != lastRoot {
				ts := time.Now()
				err := e.storeRoot()
				te := time.Now()
				if err == nil {
					log.Printf("stored root slice %s at height %d: %v", hex.EncodeToString(e.root[:]), e.ss[e.root].height, err)
					lastRoot = e.root
					ft := float64(te.Sub(ts).Nanoseconds()) / e.config.DumpDiskRatio * (1 - e.config.DumpDiskRatio)
					if ft < 3600000000000 {
						sleepTime = time.Duration(int64(ft))
					} else {
						sleepTime = time.Hour
					}
				} else {
					log.Printf("failed to store root slice %s at height %d: %v", hex.EncodeToString(e.root[:]), e.ss[e.root].height, err)
					sleepTime = time.Second * 30
				}
			}
			e.rootMut <- true
		default:
		}
	}
}

func (e *StorageEngine) Flush() error {
	e.flush <- true
	return <-e.flushed
}

func (e *StorageEngine) Stop() {
	e.stop <- true
	<-e.stopped
}
