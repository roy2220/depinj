package depinj_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/roy2220/depinj"
)

type pod1 struct {
	depinj.DummyPod
	Foo  int  `export:"Foo"`
	Foo2 *int `filter:"Foo,ModifyFoo,100"`
}

func (p *pod1) SetUp(context.Context) error {
	p.Foo = 100
	return nil
}

func (p *pod1) ModifyFoo(context.Context) error {
	*p.Foo2 -= 1
	return nil
}

type pod2 struct {
	depinj.DummyPod
	Foo int `import:"@Foo"`
	Bar int `export:""`
}

func (p *pod2) ResolveRefLink(refLink string) (string, bool) {
	if refLink == "@Foo" {
		return "Foo", true
	}

	return "", false
}

func (p *pod2) SetUp(context.Context) error {
	p.Bar = p.Foo + 2
	return nil
}

type pod3 struct {
	depinj.DummyPod
	Bar int `import:""`
	T   *testing.T
}

func (p *pod3) SetUp(context.Context) error {
	assert.Equal(p.T, 302, p.Bar)
	return nil
}

type pod4 struct {
	depinj.DummyPod
	Foo int  `import:"Foo"`
	Bar *int `filter:",ModifyBar,-1"`
}

func (p *pod4) ModifyBar(context.Context) error {
	*p.Bar += p.Foo + 1
	return nil
}

type pod5 struct {
	depinj.DummyPod
	Foo int  `import:"Foo"`
	Bar *int `filter:",ModifyBar,1"`
}

func (p *pod5) ModifyBar(context.Context) error {
	*p.Bar *= 2
	return nil
}

func TestPods(t *testing.T) {
	pp := depinj.PodPool{}
	for _, p := range []depinj.Pod{&pod5{}, &pod4{}, &pod3{T: t}, &pod2{}, &pod1{}} {
		err := pp.AddPod(p)
		assert.NoError(t, err)
	}
	err := pp.SetUp(context.Background())
	assert.NoError(t, err)
	pp.TearDown()
}

type podBase struct {
	depinj.DummyPod
	T     *testing.T
	Stack *[]*podBase
}

func (pb *podBase) SetUp(context.Context) error {
	pb.T.Logf("setup %d", len(*pb.Stack))
	*pb.Stack = append(*pb.Stack, pb)
	return nil
}

func (pb *podBase) TearDown() {
	pb.T.Logf("teardown %d", len(*pb.Stack))
	pb2 := (*pb.Stack)[len(*pb.Stack)-1]
	*pb.Stack = (*pb.Stack)[:len(*pb.Stack)-1]
	assert.True(pb.T, pb == pb2)
}

type pod6 struct {
	podBase
	Foo int `import:""`
}

func (p *pod6) SetUp(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return p.podBase.SetUp(ctx)
}

type pod7 struct {
	podBase
	Bar string `import:""`
	Foo int    `export:""`
}

type pod8 struct {
	podBase
	Baz float64 `import:""`
	Bar string  `export:""`
}

type pod9 struct {
	podBase
	Baz float64 `export:""`
}

func TestPods2(t *testing.T) {
	pp := depinj.PodPool{}
	s := []*podBase{}
	pb := podBase{T: t, Stack: &s}
	for _, p := range []depinj.Pod{&pod6{podBase: pb}, &pod7{podBase: pb}, &pod8{podBase: pb}, &pod9{podBase: pb}} {
		err := pp.AddPod(p)
		assert.NoError(t, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := pp.SetUp(ctx)
	assert.True(t, errors.Is(err, context.Canceled))
	assert.Len(t, s, 0)
}

type pod10 struct {
	podBase
	Foo *int `filter:",ModifyFoo,0"`
}

func (p *pod10) ModifyFoo(ctx context.Context) error {
	return ctx.Err()
}

func TestPods3(t *testing.T) {
	pp := depinj.PodPool{}
	s := []*podBase{}
	pb := podBase{T: t, Stack: &s}
	for _, p := range []depinj.Pod{&pod10{podBase: pb}, &pod7{podBase: pb}, &pod8{podBase: pb}, &pod9{podBase: pb}} {
		err := pp.AddPod(p)
		assert.NoError(t, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := pp.SetUp(ctx)
	assert.True(t, errors.Is(err, context.Canceled))
	assert.Len(t, s, 0)
}

type podA1 int

var _ = depinj.Pod(podA1(0))

func (podA1) ResolveRefLink(refLink string) (refID string, ok bool) { return }
func (podA1) SetUp(ctx context.Context) (err error)                 { return }
func (podA1) TearDown()                                             {}

func TestErrInvalidPod(t *testing.T) {
	p := podA1(0)

	for _, tt := range []struct {
		Pod    depinj.Pod
		Err    error
		ErrMsg string
	}{
		{podA1(0), depinj.ErrInvalidPod, "depinj: invalid pod: non-pointer type; podType=\"depinj_test.podA1\""},
		{&p, depinj.ErrInvalidPod, "depinj: invalid pod: non-structure pointer type; podType=\"*depinj_test.podA1\""},
		{&depinj.DummyPod{}, depinj.ErrInvalidPod, "depinj: invalid pod: no import/export/filter entry; podType=\"*depinj.DummyPod\""},
	} {
		pp := depinj.PodPool{}
		err := pp.AddPod(tt.Pod)
		assert.True(t, errors.Is(err, tt.Err))
		assert.EqualError(t, err, tt.ErrMsg)
	}
}

type podB1 struct {
	depinj.DummyPod
	Foo int `filter:",ModifyFoo,0"`
}

func (*podB1) ModifyFoo(context.Context) error { return nil }

type podB2 struct {
	depinj.DummyPod
	Foo *int `filter:""`
}

func (*podB2) ModifyFoo(context.Context) error { return nil }

type podB3 struct {
	depinj.DummyPod
	Foo *int `filter:",ModifyFoo,0"`
}

type podB4 struct {
	depinj.DummyPod
	Foo *int `filter:",modifyFoo,0"`
}

func (*podB4) modifyFoo(context.Context) error { return nil }

type podB5 struct {
	depinj.DummyPod
	Foo *int `filter:",ModifyFoo,0"`
}

func (*podB5) ModifyFoo() error { return nil }

type podB6 struct {
	depinj.DummyPod
	Foo *int `filter:",ModifyFoo"`
}

func (*podB6) ModifyFoo(context.Context) error { return nil }

type podB7 struct {
	depinj.DummyPod
	Foo *int `filter:",ModifyFoo,"`
}

func (*podB7) ModifyFoo(context.Context) error { return nil }

type podB8 struct {
	depinj.DummyPod
	foo *int `filter:",ModifyFoo,0"`
}

func (*podB8) ModifyFoo(context.Context) error { return nil }

type podB9 struct {
	depinj.DummyPod
	foo int `import:""`
}

type podB10 struct {
	depinj.DummyPod
	foo int `export:""`
}

func TestFieldParseFailed(t *testing.T) {
	for _, tt := range []struct {
		Pod    depinj.Pod
		Err    error
		ErrMsg string
	}{
		{&podB1{}, depinj.ErrBadFilterEntry, "depinj: bad filter entry: non-pointer field type; filterEntryPath=\"depinj_test.podB1.Foo\" fieldType=\"int\""},
		{&podB2{}, depinj.ErrBadFilterEntry, "depinj: bad filter entry: missing argument `methodName`; filterEntryPath=\"depinj_test.podB2.Foo\""},
		{&podB3{}, depinj.ErrBadFilterEntry, "depinj: bad filter entry: method undefined or unexported; filterEntryPath=\"depinj_test.podB3.Foo\" methodName=\"ModifyFoo\""},
		{&podB4{}, depinj.ErrBadFilterEntry, "depinj: bad filter entry: method undefined or unexported; filterEntryPath=\"depinj_test.podB4.Foo\" methodName=\"modifyFoo\""},
		{&podB5{}, depinj.ErrBadFilterEntry, "depinj: bad filter entry: function type mismatch (expected `func(context.Context) error`, got `func() error`); filterEntryPath=\"depinj_test.podB5.Foo\" methodName=\"ModifyFoo\""},
		{&podB6{}, depinj.ErrBadFilterEntry, "depinj: bad filter entry: missing argument `priority`; filterEntryPath=\"depinj_test.podB6.Foo\""},
		{&podB7{}, depinj.ErrBadFilterEntry, "depinj: bad filter entry: priority parse failed; filterEntryPath=\"depinj_test.podB7.Foo\" priorityStr=\"\": strconv.Atoi: parsing \"\": invalid syntax"},
		{&podB8{}, depinj.ErrBadFilterEntry, "depinj: bad filter entry: field unexported; filterEntryPath=\"depinj_test.podB8.foo\""},
		{&podB9{}, depinj.ErrBadImportEntry, "depinj: bad import entry: field unexported; importEntryPath=\"depinj_test.podB9.foo\""},
		{&podB10{}, depinj.ErrBadExportEntry, "depinj: bad export entry: field unexported; exportEntryPath=\"depinj_test.podB10.foo\""},
	} {
		pp := depinj.PodPool{}
		err := pp.AddPod(tt.Pod)
		assert.True(t, errors.Is(err, tt.Err))
		assert.EqualError(t, err, tt.ErrMsg)
	}
}

type podC1 struct {
	depinj.DummyPod
	Foo int `import:"@Foo"`
}

type podC2 struct {
	depinj.DummyPod
	Foo int `export:"@Foo"`
}

type podC3 struct {
	depinj.DummyPod
	Foo *int `filter:"@Foo,ModifyFoo,-1"`
}

func (*podC3) ModifyFoo(context.Context) error { return nil }

type podC4 struct {
	depinj.DummyPod
	Foo int `export:""`
}

type podC5 struct {
	podC4
}

type podC6 struct {
	depinj.DummyPod
	Foo int `export:"Foo"`
}

type podC7 struct {
	podC6
}

func (*podC4) ResolveRefLink(string) (string, bool) { return "foo", true }

func TestEntryResolve1Failed(t *testing.T) {
	for _, tt := range []struct {
		Pods   []depinj.Pod
		Err    error
		ErrMsg string
	}{
		{[]depinj.Pod{&podC1{}}, depinj.ErrBadImportEntry, "depinj: bad import entry: unresolvable ref link; importEntryPath=\"depinj_test.podC1.Foo\" refLink=\"@Foo\""},
		{[]depinj.Pod{&podC2{}}, depinj.ErrBadExportEntry, "depinj: bad export entry: unresolvable ref link; exportEntryPath=\"depinj_test.podC2.Foo\" refLink=\"@Foo\""},
		{[]depinj.Pod{&podC3{}}, depinj.ErrBadFilterEntry, "depinj: bad filter entry: unresolvable ref link; filterEntryPath=\"depinj_test.podC3.Foo\" refLink=\"@Foo\""},
		{[]depinj.Pod{&podC4{}, &podC5{}}, depinj.ErrBadExportEntry, "depinj: bad export entry: duplicate field type; exportEntryPath=\"depinj_test.podC5.podC4.Foo\" conflictingExportEntryPath=\"depinj_test.podC4.Foo\" fieldType=\"int\""},
		{[]depinj.Pod{&podC5{}, &podC5{}}, depinj.ErrBadExportEntry, "depinj: bad export entry: duplicate field type; exportEntryPath=\"depinj_test.podC5.podC4.Foo\" conflictingExportEntryPath=\"depinj_test.podC5.podC4.Foo\" fieldType=\"int\""},
		{[]depinj.Pod{&podC6{}, &podC7{}}, depinj.ErrBadExportEntry, "depinj: bad export entry: duplicate ref id; exportEntryPath=\"depinj_test.podC7.podC6.Foo\" conflictingExportEntryPath=\"depinj_test.podC6.Foo\" refID=\"Foo\""},
		{[]depinj.Pod{&podC7{}, &podC7{}}, depinj.ErrBadExportEntry, "depinj: bad export entry: duplicate ref id; exportEntryPath=\"depinj_test.podC7.podC6.Foo\" conflictingExportEntryPath=\"depinj_test.podC7.podC6.Foo\" refID=\"Foo\""},
	} {
		pp := depinj.PodPool{}
		for _, p := range tt.Pods {
			err := pp.AddPod(p)
			assert.NoError(t, err)
		}
		err := pp.SetUp(context.Background())
		assert.True(t, errors.Is(err, tt.Err))
		assert.EqualError(t, err, tt.ErrMsg)
		err = pp.SetUp(context.Background())
		assert.True(t, errors.Is(err, tt.Err))
		assert.EqualError(t, err, tt.ErrMsg)
		pp.TearDown()
	}
}

type podD1 struct {
	depinj.DummyPod
	Foo int `import:""`
}

type podD2 struct {
	depinj.DummyPod
	Foo int `import:"Foo"`
}

type podD3 struct {
	depinj.DummyPod
	Foo *int `filter:",ModifyFoo,-1"`
}

func (*podD3) ModifyFoo(context.Context) error { return nil }

type podD4 struct {
	depinj.DummyPod
	Foo *int `filter:"Foo,ModifyFoo,-1"`
}

func (*podD4) ModifyFoo(context.Context) error { return nil }

type podD5 struct {
	depinj.DummyPod
	Foo int `import:"Foo"`
}

type podD6 struct {
	depinj.DummyPod
	Foo string `export:"Foo"`
}

type podD7 struct {
	depinj.DummyPod
	Foo *int `filter:"Foo,ModifyFoo,-1"`
}

func (*podD7) ModifyFoo(context.Context) error { return nil }

func TestEntryResolve2Failed(t *testing.T) {
	for _, tt := range []struct {
		Pods   []depinj.Pod
		Err    error
		ErrMsg string
	}{
		{[]depinj.Pod{&podD1{}}, depinj.ErrBadImportEntry, "depinj: bad import entry: export entry not found by field type; importEntryPath=\"depinj_test.podD1.Foo\" fieldType=\"int\""},
		{[]depinj.Pod{&podD2{}}, depinj.ErrBadImportEntry, "depinj: bad import entry: export entry not found by ref id; importEntryPath=\"depinj_test.podD2.Foo\" refID=\"Foo\""},
		{[]depinj.Pod{&podD3{}}, depinj.ErrBadFilterEntry, "depinj: bad filter entry: export entry not found by field type; filterEntryPath=\"depinj_test.podD3.Foo\" fieldType=\"int\""},
		{[]depinj.Pod{&podD4{}}, depinj.ErrBadFilterEntry, "depinj: bad filter entry: export entry not found by ref id; filterEntryPath=\"depinj_test.podD4.Foo\" refID=\"Foo\""},
		{[]depinj.Pod{&podD5{}, &podD6{}}, depinj.ErrBadImportEntry, "depinj: bad import entry: field type mismatch; importEntryPath=\"depinj_test.podD5.Foo\" fieldType=\"int\" expectedFieldType=\"string\" exportEntryPath=\"depinj_test.podD6.Foo\""},
		{[]depinj.Pod{&podD7{}, &podD6{}}, depinj.ErrBadFilterEntry, "depinj: bad filter entry: field type mismatch; filterEntryPath=\"depinj_test.podD7.Foo\" fieldType=\"*int\" expectedFieldType=\"*string\" exportEntryPath=\"depinj_test.podD6.Foo\""},
	} {
		pp := depinj.PodPool{}
		for _, p := range tt.Pods {
			err := pp.AddPod(p)
			assert.NoError(t, err)
		}
		err := pp.SetUp(context.Background())
		assert.True(t, errors.Is(err, tt.Err))
		assert.EqualError(t, err, tt.ErrMsg)
		err = pp.SetUp(context.Background())
		assert.True(t, errors.Is(err, tt.Err))
		assert.EqualError(t, err, tt.ErrMsg)
		pp.TearDown()
	}
}

type podE1 struct {
	depinj.DummyPod
	FooE int `export:""`
	FooI int `import:""`
}

type podE2 struct {
	depinj.DummyPod
	Foo int    `import:""`
	Bar string `export:""`
}

type podE3 struct {
	depinj.DummyPod
	Bar string `import:""`
	Foo int    `export:""`
}

type podE4 struct {
	depinj.DummyPod
	Bar string `export:""`
	Foo *int   `filter:",ModifyFoo,-1"`
}

func (*podE4) ModifyFoo(context.Context) error { return nil }

type podE5 struct {
	depinj.DummyPod
	Foo int     `export:""`
	Bar *string `filter:",ModifyBar,-1"`
}

func (*podE5) ModifyBar(context.Context) error { return nil }

type podE6 struct {
	depinj.DummyPod
	Foo int    `export:""`
	Bar string `export:""`
}

type podE7 struct {
	depinj.DummyPod
	Foo int     `import:""`
	Bar *string `filter:",ModifyBar,-1"`
}

func (*podE7) ModifyBar(context.Context) error { return nil }

func TestEntryResolve3Failed(t *testing.T) {
	for _, tt := range []struct {
		Pods   []depinj.Pod
		Err    error
		ErrMsg string
	}{
		{[]depinj.Pod{&podE1{}}, depinj.ErrPodCircularDependency, "depinj: pod circular dependency; stackTrace=\"depinj_test.podE1.FooI ==> depinj_test.podE1.FooE\""},
		{[]depinj.Pod{&podE2{}, &podE3{}}, depinj.ErrPodCircularDependency, "depinj: pod circular dependency; stackTrace=\"depinj_test.podE2.Foo ==> depinj_test.podE3.Foo ... depinj_test.podE3.Bar ==> depinj_test.podE2.Bar\""},
		{[]depinj.Pod{&podE4{}, &podE5{}}, depinj.ErrPodCircularDependency, "depinj: pod circular dependency; stackTrace=\"depinj_test.podE4.Bar ==> depinj_test.podE5.Bar ... depinj_test.podE5.Foo ==> depinj_test.podE4.Foo\""},
		{[]depinj.Pod{&podE6{}, &podE7{}}, depinj.ErrPodCircularDependency, "depinj: pod circular dependency; stackTrace=\"depinj_test.podE6.Bar ==> depinj_test.podE7.Bar ... depinj_test.podE7.Foo ==> depinj_test.podE6.Foo\""},
	} {
		pp := depinj.PodPool{}
		for _, p := range tt.Pods {
			err := pp.AddPod(p)
			assert.NoError(t, err)
		}
		err := pp.SetUp(context.Background())
		assert.True(t, errors.Is(err, tt.Err))
		assert.EqualError(t, err, tt.ErrMsg)
		err = pp.SetUp(context.Background())
		assert.True(t, errors.Is(err, tt.Err))
		assert.EqualError(t, err, tt.ErrMsg)
		pp.TearDown()
	}
}
