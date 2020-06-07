// Package depinj implements dependency injection.
package depinj

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// PodPool represents a set of pods.
type PodPool struct {
	pods     []pod
	firstPod *pod
	lastPod  *pod
}

// AddPod adds the given pod to the pool.
func (pp *PodPool) AddPod(rawPod Pod) error {
	var pod pod

	if err := pod.ParseRaw(rawPod); err != nil {
		return err
	}

	pp.pods = append(pp.pods, pod)
	return nil
}

// MustAddPod adds the given pod to the pool, it panics if any error occurs.
func (pp *PodPool) MustAddPod(rawPod Pod) {
	if err := pp.AddPod(rawPod); err != nil {
		panic(err)
	}
}

// SetUp sets up all the pods in the pool.
func (pp *PodPool) SetUp(ctx context.Context) (returnedErr error) {
	if err := pp.resolve(); err != nil {
		return err
	}

	pod := pp.firstPod

	defer func() {
		if returnedErr != nil {
			for pod = pod.Prev; pod != nil; pod = pod.Prev {
				pod.TearDown()
			}
		}
	}()

	for ; pod != nil; pod = pod.Next {
		if err := pod.SetUp(ctx); err != nil {
			return err
		}
	}

	return nil
}

// MustSetUp sets up all the pods in the pool, it panics if any error occurs.
func (pp *PodPool) MustSetUp(ctx context.Context) {
	if err := pp.SetUp(ctx); err != nil {
		panic(err)
	}
}

// TearDown tears down all the pods in the pool in a reverse order of setups.
func (pp *PodPool) TearDown() {
	for pod := pp.lastPod; pod != nil; pod = pod.Prev {
		pod.TearDown()
	}
}

func (pp *PodPool) resolve() error {
	{
		context := new(resolution12Context).Init()

		for i := range pp.pods {
			pod := &pp.pods[i]

			if err := pod.Resolve1(context); err != nil {
				return err
			}
		}

		for i := range pp.pods {
			pod := &pp.pods[i]

			if err := pod.Resolve2(context); err != nil {
				return err
			}
		}
	}

	{
		context := new(resolution3Context).Init()

		for i := range pp.pods {
			pod := &pp.pods[i]

			if err := pod.Resolve3(context); err != nil {
				return err
			}
		}

		pp.firstPod = context.FirstPod()
		pp.lastPod = context.LastPod()
	}

	return nil
}

// Pod represents a container for dependency injection.
type Pod interface {
	// ResolveRefLink resolves the given ref link into a ref id.
	// It returns false if the ref link is unresolvable. When it
	// is called, the fields of the import/export/filter entries
	// have not yet been initialized, don't access theme.
	ResolveRefLink(refLink string) (refID string, ok bool)

	// SetUp is called along with the setup of PodPool. When it
	// is called, all the fields of the import entries have been
	// initialized. The fields of the export entries should be
	// initialized within this method.
	SetUp(ctx context.Context) (err error)

	// TearDown is called along with the teardown of PodPool.
	// The fields of the export entries should be finalized
	// as necessary within this method.
	TearDown()
}

// DummyPod is the dummy implementation of Pod.
// It could be embedded as the default implementation of Pod.
type DummyPod struct{}

var _ = Pod(DummyPod{})

// ResolveRefLink does nothing.
func (DummyPod) ResolveRefLink(refLink string) (refID string, ok bool) { return }

// SetUp does nothing.
func (DummyPod) SetUp(ctx context.Context) (err error) { return }

// TearDown does nothing.
func (DummyPod) TearDown() {}

// Sentinel errors
var (
	ErrInvalidPod            = errors.New("depinj: invalid pod")
	ErrBadImportEntry        = errors.New("depinj: bad import entry")
	ErrBadExportEntry        = errors.New("depinj: bad export entry")
	ErrBadFilterEntry        = errors.New("depinj: bad filter entry")
	ErrPodCircularDependency = errors.New("depinj: pod circular dependency")
)

const (
	resolution3PodEntered = resolution3PodState(1 + iota)
	resolution3PodLeft
)

type pod struct {
	// ParseRaw
	Raw           Pod
	ImportEntries []importEntry
	ExportEntries []exportEntry
	FilterEntries []filterEntry

	// Resolve3
	Next *pod
	Prev *pod
}

func (p *pod) ParseRaw(raw Pod) error {
	p.Raw = raw
	value := reflect.ValueOf(raw)

	if value.Kind() != reflect.Ptr {
		return fmt.Errorf("%w: non-pointer type: podType=%q", ErrInvalidPod, value.Type())
	}

	structureValue := value.Elem()

	if structureValue.Kind() != reflect.Struct {
		return fmt.Errorf("%w: non-structure pointer type: podType=%q", ErrInvalidPod, value.Type())
	}

	if err := p.parseStructure(nil, structureValue); err != nil {
		return err
	}

	if len(p.ImportEntries)+len(p.ExportEntries)+len(p.FilterEntries) == 0 {
		return fmt.Errorf("%w: no import/export/filter entry: podType=%q", ErrInvalidPod, value.Type())
	}

	return nil
}

func (p *pod) Resolve1(context *resolution12Context) error {
	for i := range p.ImportEntries {
		importEntry := &p.ImportEntries[i]

		if err := importEntry.Resolve1(p); err != nil {
			return err
		}
	}

	for i := range p.ExportEntries {
		exportEntry := &p.ExportEntries[i]

		if err := exportEntry.Resolve1(context, p); err != nil {
			return err
		}
	}

	for i := range p.FilterEntries {
		filterEntry := &p.FilterEntries[i]

		if err := filterEntry.Resolve1(p); err != nil {
			return err
		}
	}

	return nil
}

func (p *pod) Resolve2(context *resolution12Context) error {
	for i := range p.ImportEntries {
		importEntry := &p.ImportEntries[i]

		if err := importEntry.Resolve2(context); err != nil {
			return err
		}
	}

	for i := range p.FilterEntries {
		filterEntry := &p.FilterEntries[i]

		if err := filterEntry.Resolve2(context); err != nil {
			return err
		}
	}

	return nil
}

func (p *pod) Resolve3(context *resolution3Context) error {
	return p.doResolve3(context, "")
}

func (p *pod) SetUp(ctx context.Context) (returnedErr error) {
	for i := range p.ImportEntries {
		importEntry := &p.ImportEntries[i]
		exportEntry := importEntry.ExportEntry
		importEntry.FieldValue.Set(exportEntry.FieldValue)
	}

	if err := p.Raw.SetUp(ctx); err != nil {
		return fmt.Errorf("depinj: pod setup failed: pod=%#v | %w", p.Raw, err)
	}

	defer func() {
		if returnedErr != nil {
			p.TearDown()
		}
	}()

	for i := range p.ExportEntries {
		exportEntry := &p.ExportEntries[i]

		for _, filterEntry := range exportEntry.FilterEntries {
			filterEntry.FieldValue.Set(exportEntry.FieldValue.Addr())
		}

		for _, filterEntry := range exportEntry.FilterEntries {
			if err := filterEntry.Function(ctx); err != nil {
				return fmt.Errorf("depinj: filter function failed: pod=%#v | %w", p.Raw, err)
			}
		}
	}

	return nil
}

func (p *pod) TearDown() {
	p.Raw.TearDown()

	for i := range p.ImportEntries {
		importEntry := &p.ImportEntries[i]
		importEntry.FieldValue.Set(reflect.Zero(importEntry.FieldType))
	}

	for i := range p.ExportEntries {
		exportEntry := &p.ExportEntries[i]
		exportEntry.FieldValue.Set(reflect.Zero(exportEntry.FieldType))
	}

	for i := range p.FilterEntries {
		filterEntry := &p.FilterEntries[i]
		filterEntry.FieldValue.Set(reflect.Zero(filterEntry.FieldType))
	}
}

func (p *pod) parseStructure(parentFieldInfo *fieldInfo, structureValue reflect.Value) error {
	fieldInfo := fieldInfo{
		Parent:         parentFieldInfo,
		StructureValue: structureValue,
		StructureType:  structureValue.Type(),
	}

	for i, n := 0, fieldInfo.StructureType.NumField(); i < n; i++ {
		fieldInfo.Descriptor = fieldInfo.StructureType.Field(i)

		if fieldInfo.Descriptor.Anonymous && fieldInfo.Descriptor.Type.Kind() == reflect.Struct {
			p.parseStructure(&fieldInfo, structureValue.Field(i))
			continue
		}

		var importEntry importEntry

		if ok, err := importEntry.ParseField(&fieldInfo); ok {
			p.ImportEntries = append(p.ImportEntries, importEntry)
			continue
		} else if err != nil {
			return err
		}

		var exportEntry exportEntry

		if ok, err := exportEntry.ParseField(&fieldInfo); ok {
			p.ExportEntries = append(p.ExportEntries, exportEntry)
			continue
		} else if err != nil {
			return err
		}

		var filterEntry filterEntry

		if ok, err := filterEntry.ParseField(&fieldInfo); ok {
			p.FilterEntries = append(p.FilterEntries, filterEntry)
			continue
		} else if err != nil {
			return err
		}
	}

	return nil
}

func (p *pod) doResolve3(context *resolution3Context, targetEntryPath string) error {
	switch podState := context.EnterPod(p, targetEntryPath); podState {
	case resolution3PodLeft:
		context.LeavePod()
		return nil
	case resolution3PodEntered:
		return fmt.Errorf("%w: stackTrace=%q", ErrPodCircularDependency, context.DumpStack())
	}

	for i := range p.ImportEntries {
		importEntry := &p.ImportEntries[i]
		context.SetActiveEntryPath(importEntry.Path)
		exportEntry := importEntry.ExportEntry

		if err := exportEntry.Pod.doResolve3(context, exportEntry.Path); err != nil {
			return err
		}
	}

	for i := range p.ExportEntries {
		exportEntry := &p.ExportEntries[i]
		context.SetActiveEntryPath(exportEntry.Path)

		sort.Slice(exportEntry.FilterEntries, func(i, j int) bool {
			return exportEntry.FilterEntries[i].Priority >= exportEntry.FilterEntries[j].Priority
		})

		for _, filterEntry := range exportEntry.FilterEntries {
			if filterEntry.Pod == p {
				continue
			}

			if err := filterEntry.Pod.doResolve3(context, filterEntry.Path); err != nil {
				return err
			}
		}
	}

	context.LeavePod()
	context.AppendPod(p)
	return nil
}

type fieldInfo struct {
	Parent         *fieldInfo
	StructureValue reflect.Value
	StructureType  reflect.Type
	Descriptor     reflect.StructField
}

func (fi *fieldInfo) Path() string {
	if fi.Parent == nil {
		return fi.StructureType.String() + "." + fi.Descriptor.Name
	}

	return fi.Parent.Path() + "." + fi.Descriptor.Name
}

type entry struct {
	// ParseField
	Path       string
	FieldValue reflect.Value
	FieldType  reflect.Type
	RefID      string
}

func (e *entry) ParseField(fieldInfo *fieldInfo, fieldTagKey string) ([]string, bool) {
	fieldTagKey, ok := fieldInfo.Descriptor.Tag.Lookup(fieldTagKey)

	if !ok {
		return nil, false
	}

	e.Path = fieldInfo.Path()
	e.FieldValue = fieldInfo.StructureValue.Field(fieldInfo.Descriptor.Index[0])
	e.FieldType = fieldInfo.Descriptor.Type
	args := strings.Split(fieldTagKey, ",")
	e.RefID = args[0]
	return args, true
}

func (e *entry) ResolveRefLink(pod *pod) (string, bool) {
	if refLink := e.RefID; isRefLink(refLink) {
		refID, ok := pod.Raw.ResolveRefLink(refLink)

		if !ok {
			return refLink, false
		}

		e.RefID = refID
	}

	return "", true
}

type importEntry struct {
	entry

	// Resolve1
	Pod *pod

	// Resolve2
	ExportEntry *exportEntry
}

func (ie *importEntry) ParseField(fieldInfo *fieldInfo) (bool, error) {
	_, ok := ie.entry.ParseField(fieldInfo, "import")

	if !ok {
		return false, nil
	}

	if fieldInfo.Descriptor.PkgPath != "" {
		return false, fmt.Errorf("%w: field unexported: importEntryPath=%q",
			ErrBadImportEntry, ie.Path)
	}

	return true, nil
}

func (ie *importEntry) Resolve1(pod *pod) error {
	ie.Pod = pod

	if refLink, ok := ie.ResolveRefLink(pod); !ok {
		return fmt.Errorf("%w: unresolvable ref link: importEntryPath=%q refLink=%q",
			ErrBadImportEntry, ie.Path, refLink)
	}

	return nil
}

func (ie *importEntry) Resolve2(context *resolution12Context) error {
	if ie.RefID == "" {
		var ok bool
		ie.ExportEntry, ok = context.FindExportEntryByFieldType(ie.FieldType)

		if !ok {
			return fmt.Errorf("%w: export entry not found by field type: importEntryPath=%q fieldType=%q",
				ErrBadImportEntry, ie.Path, ie.FieldType)
		}
	} else {
		var ok bool
		ie.ExportEntry, ok = context.FindExportEntryByRefID(ie.RefID)

		if !ok {
			return fmt.Errorf("%w: export entry not found by ref id: importEntryPath=%q refID=%q",
				ErrBadImportEntry, ie.Path, ie.RefID)
		}

		if expectedFieldType := ie.ExportEntry.FieldType; ie.FieldType != expectedFieldType {
			return fmt.Errorf("%w: field type mismatch: importEntryPath=%q fieldType=%q expectedFieldType=%q exportEntryPath=%q",
				ErrBadImportEntry, ie.Path, ie.FieldType, expectedFieldType, ie.ExportEntry.Path)
		}
	}

	return nil
}

type exportEntry struct {
	entry

	// Resolve1
	Pod *pod

	// Resolve2
	FilterEntries []*filterEntry
}

func (ee *exportEntry) ParseField(fieldInfo *fieldInfo) (bool, error) {
	_, ok := ee.entry.ParseField(fieldInfo, "export")

	if !ok {
		return false, nil
	}

	if fieldInfo.Descriptor.PkgPath != "" {
		return false, fmt.Errorf("%w: field unexported: exportEntryPath=%q",
			ErrBadExportEntry, ee.Path)
	}

	return true, nil
}

func (ee *exportEntry) Resolve1(context *resolution12Context, pod *pod) error {
	ee.Pod = pod

	if refLink, ok := ee.ResolveRefLink(pod); !ok {
		return fmt.Errorf("%w: unresolvable ref link: exportEntryPath=%q refLink=%q",
			ErrBadExportEntry, ee.Path, refLink)
	}

	if ee.RefID == "" {
		if conflicting, ok := context.AddExportEntryByFieldType(ee, ee.FieldType); !ok {
			return fmt.Errorf("%w: duplicate field type: exportEntryPath=%q conflictingExportEntryPath=%q fieldType=%q",
				ErrBadExportEntry, ee.Path, conflicting.Path, ee.FieldType)
		}
	} else {
		if conflicting, ok := context.AddExportEntryByRefID(ee, ee.RefID); !ok {
			return fmt.Errorf("%w: duplicate ref id: exportEntryPath=%q conflictingExportEntryPath=%q refID=%q",
				ErrBadExportEntry, ee.Path, conflicting.Path, ee.RefID)
		}
	}

	return nil
}

type filterEntry struct {
	entry

	// ParseField
	Function func(context.Context) error
	Priority int

	// Resolve1
	Pod *pod
}

func (fe *filterEntry) ParseField(fieldInfo *fieldInfo) (bool, error) {
	args, ok := fe.entry.ParseField(fieldInfo, "filter")

	if !ok {
		return false, nil
	}

	if fieldInfo.Descriptor.PkgPath != "" {
		return false, fmt.Errorf("%w: field unexported: filterEntryPath=%q",
			ErrBadFilterEntry, fe.Path)
	}

	if fe.FieldType.Kind() != reflect.Ptr {
		return false, fmt.Errorf("%w: non-pointer field type: filterEntryPath=%q fieldType=%q",
			ErrBadFilterEntry, fe.Path, fe.FieldType)
	}

	if len(args) < 2 {
		return false, fmt.Errorf("%w: missing argument `methodName`: filterEntryPath=%q",
			ErrBadFilterEntry, fe.Path)
	}

	methodName := args[1]
	functionValue := fieldInfo.StructureValue.Addr().MethodByName(methodName)

	if !functionValue.IsValid() {
		return false, fmt.Errorf("%w: method undefined or unexported: filterEntryPath=%q methodName=%q",
			ErrBadFilterEntry, fe.Path, methodName)
	}

	rawFunction := functionValue.Interface()
	fe.Function, ok = rawFunction.(func(context.Context) error)

	if !ok {
		return false, fmt.Errorf("%w: function type mismatch (expected `%T`, got `%T`): filterEntryPath=%q methodName=%q",
			ErrBadFilterEntry, fe.Function, rawFunction, fe.Path, methodName)
	}

	if len(args) < 3 {
		return false, fmt.Errorf("%w: missing argument `priority`: filterEntryPath=%q",
			ErrBadFilterEntry, fe.Path)
	}

	priorityStr := args[2]
	var err error
	fe.Priority, err = strconv.Atoi(priorityStr)

	if err != nil {
		return false, fmt.Errorf("%w: priority parse failed: filterEntryPath=%q priorityStr=%q | %v",
			ErrBadFilterEntry, fe.Path, priorityStr, err)
	}

	return true, nil
}

func (fe *filterEntry) Resolve1(pod *pod) error {
	fe.Pod = pod

	if refLink, ok := fe.ResolveRefLink(pod); !ok {
		return fmt.Errorf("%w: unresolvable ref link: filterEntryPath=%q refLink=%q",
			ErrBadFilterEntry, fe.Path, refLink)
	}

	return nil
}

func (fe *filterEntry) Resolve2(context *resolution12Context) error {
	var exportEntry *exportEntry

	if fe.RefID == "" {
		fieldType := fe.FieldType.Elem()
		var ok bool
		exportEntry, ok = context.FindExportEntryByFieldType(fieldType)

		if !ok {
			return fmt.Errorf("%w: export entry not found by field type: filterEntryPath=%q fieldType=%q",
				ErrBadFilterEntry, fe.Path, fieldType)
		}
	} else {
		var ok bool
		exportEntry, ok = context.FindExportEntryByRefID(fe.RefID)

		if !ok {
			return fmt.Errorf("%w: export entry not found by ref id: filterEntryPath=%q refID=%q",
				ErrBadFilterEntry, fe.Path, fe.RefID)
		}

		if expectedFieldType := reflect.PtrTo(exportEntry.FieldType); fe.FieldType != expectedFieldType {
			return fmt.Errorf("%w: field type mismatch: filterEntryPath=%q fieldType=%q expectedFieldType=%q exportEntryPath=%q",
				ErrBadFilterEntry, fe.Path, fe.FieldType, expectedFieldType, exportEntry.Path)
		}
	}

	// ensure idempotence
	for _, other := range exportEntry.FilterEntries {
		if other == fe {
			return nil
		}
	}

	exportEntry.FilterEntries = append(exportEntry.FilterEntries, fe)
	return nil
}

type resolution12Context struct {
	fieldType2ExportEntry map[reflect.Type]*exportEntry
	refID2ExportEntry     map[string]*exportEntry
}

func (rc *resolution12Context) Init() *resolution12Context {
	rc.fieldType2ExportEntry = map[reflect.Type]*exportEntry{}
	rc.refID2ExportEntry = map[string]*exportEntry{}
	return rc
}

func (rc *resolution12Context) AddExportEntryByFieldType(exportEntry *exportEntry, fieldType reflect.Type) (*exportEntry, bool) {
	if addedExportEntry, ok := rc.fieldType2ExportEntry[fieldType]; ok {
		return addedExportEntry, false
	}

	rc.fieldType2ExportEntry[fieldType] = exportEntry
	return nil, true
}

func (rc *resolution12Context) AddExportEntryByRefID(exportEntry *exportEntry, refID string) (*exportEntry, bool) {
	if addedExportEntry, ok := rc.refID2ExportEntry[refID]; ok {
		return addedExportEntry, false
	}

	rc.refID2ExportEntry[refID] = exportEntry
	return nil, true
}

func (rc *resolution12Context) FindExportEntryByFieldType(fieldType reflect.Type) (*exportEntry, bool) {
	exportEntry, ok := rc.fieldType2ExportEntry[fieldType]
	return exportEntry, ok
}

func (rc *resolution12Context) FindExportEntryByRefID(refID string) (*exportEntry, bool) {
	exportEntry, ok := rc.refID2ExportEntry[refID]
	return exportEntry, ok
}

type resolution3Context struct {
	stack     []resolution3StackFrame
	podStates map[*pod]resolution3PodState

	firstPod *pod
	lastPod  *pod
}

func (rc *resolution3Context) Init() *resolution3Context {
	rc.podStates = map[*pod]resolution3PodState{}
	return rc
}

func (rc *resolution3Context) EnterPod(pod *pod, targetEntryPath string) resolution3PodState {
	rc.stack = append(rc.stack, resolution3StackFrame{
		Pod:             pod,
		TargetEntryPath: targetEntryPath,
	})

	podState := rc.podStates[pod]
	rc.podStates[pod] = resolution3PodEntered
	return podState
}

func (rc *resolution3Context) LeavePod() {
	pod := rc.stack[len(rc.stack)-1].Pod
	rc.stack = rc.stack[:len(rc.stack)-1]
	rc.podStates[pod] = resolution3PodLeft
}

func (rc *resolution3Context) SetActiveEntryPath(activeEntryPath string) {
	rc.stack[len(rc.stack)-1].ActiveEntryPath = activeEntryPath
}

func (rc *resolution3Context) DumpStack() string {
	stackTraceBuffer := bytes.Buffer{}

	for i, stackFrame := range rc.stack {
		if i >= 1 {
			stackTraceBuffer.WriteString(" ==> ")
		}

		f1 := stackFrame.TargetEntryPath != ""
		f2 := stackFrame.ActiveEntryPath != ""

		if f1 {
			stackTraceBuffer.WriteString(stackFrame.TargetEntryPath)
		}

		if f1 && f2 {
			stackTraceBuffer.WriteString(" ... ")
		}

		if f2 {
			stackTraceBuffer.WriteString(stackFrame.ActiveEntryPath)
		}
	}

	return stackTraceBuffer.String()
}

func (rc *resolution3Context) AppendPod(pod *pod) {
	pod.Next = nil // ensure idempotence
	pod.Prev = rc.lastPod
	rc.lastPod = pod

	if pod.Prev == nil {
		rc.firstPod = pod
	} else {
		pod.Prev.Next = pod
	}
}

func (rc *resolution3Context) FirstPod() *pod {
	return rc.firstPod
}

func (rc *resolution3Context) LastPod() *pod {
	return rc.lastPod
}

type resolution3StackFrame struct {
	Pod             *pod
	TargetEntryPath string
	ActiveEntryPath string
}

type resolution3PodState int

func isRefLink(refLink string) bool {
	return len(refLink) >= 1 && refLink[0] == '@'
}
