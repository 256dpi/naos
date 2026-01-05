package msg

import (
	"errors"
	"iter"
	"time"
)

var ErrParamNotFound = errors.New("param not found")

type ParamsService struct {
	session *Session
	infos   []ParamInfo
	byName  map[string]ParamInfo
	byRef   map[uint8]ParamInfo
	updates map[uint8]ParamUpdate
}

func NewParamsService(session *Session) *ParamsService {
	return &ParamsService{
		session: session,
		byName:  make(map[string]ParamInfo),
		byRef:   make(map[uint8]ParamInfo),
		updates: make(map[uint8]ParamUpdate),
	}
}

func (s *ParamsService) List() error {
	// list params
	infos, err := ListParams(s.session, 5*time.Second)
	if err != nil {
		return err
	}

	// store infos
	s.infos = infos
	for _, param := range infos {
		s.byName[param.Name] = param
		s.byRef[param.Ref] = param
	}

	return nil
}

func (s *ParamsService) Has(name string) bool {
	_, ok := s.byName[name]
	return ok
}

func (s *ParamsService) Get(name string) (ParamInfo, bool) {
	param, ok := s.byName[name]
	return param, ok
}

func (s *ParamsService) All() iter.Seq2[ParamInfo, ParamUpdate] {
	return func(yield func(ParamInfo, ParamUpdate) bool) {
		for _, info := range s.infos {
			update, _ := s.updates[info.Ref]
			if !yield(info, update) {
				return
			}
		}
	}
}

func (s *ParamsService) Collect(names ...string) error {
	// prepare refs
	var refs []uint8
	if len(names) > 0 {
		refs = make([]uint8, len(names))
		for i, name := range names {
			param, ok := s.byName[name]
			if !ok {
				return ErrParamNotFound
			}
			refs[i] = param.Ref
		}
	} else {
		refs = make([]uint8, len(s.infos))
		for i, info := range s.infos {
			refs[i] = info.Ref
		}
	}

	// find max age for refs
	var maxAge uint64
	for _, ref := range refs {
		update, ok := s.updates[ref]
		if ok {
			maxAge = max(maxAge, update.Age)
		} else {
			maxAge = 0
			break
		}
	}

	// collect params
	updates, err := CollectParams(s.session, refs, maxAge, 5*time.Second)
	if err != nil {
		return err
	}

	// store updates
	for _, update := range updates {
		s.updates[update.Ref] = update
	}

	return nil
}

func (s *ParamsService) Read(name string, reload bool) ([]byte, error) {
	// find param
	param, ok := s.byName[name]
	if !ok {
		return nil, ErrParamNotFound
	}

	// reload param
	if reload {
		update, err := CollectParams(s.session, []uint8{param.Ref}, 0, 5*time.Second)
		if err != nil {
			return nil, err
		}
		s.updates[param.Ref] = update[0]
	}

	// find update
	update, ok := s.updates[param.Ref]
	if !ok {
		return nil, ErrParamNotFound
	}

	return update.Value, nil
}

func (s *ParamsService) Write(name string, value []byte) error {
	// find param
	param, ok := s.byName[name]
	if !ok {
		return ErrParamNotFound
	}

	// write param
	err := WriteParam(s.session, param.Ref, value, 5*time.Second)
	if err != nil {
		return err
	}

	return nil
}
