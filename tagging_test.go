package tagging

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/loadbalance"
)

type mockBalancer struct {
	rebalanced bool
	deleted    bool
}

type mockPicker struct {
	result discovery.Result
}

func (m *mockBalancer) GetPicker(result discovery.Result) loadbalance.Picker {
	return &mockPicker{result: result}
}

func (m *mockBalancer) Rebalance(change discovery.Change) {
	m.rebalanced = true
}

func (m *mockBalancer) Delete(change discovery.Change) {
	m.deleted = true
}

func (m *mockPicker) Next(ctx context.Context, request interface{}) discovery.Instance {
	return m.result.Instances[0]
}

func (m *mockBalancer) Name() string {
	return "mock"
}

func TestTaggingBalancer_Name(t *testing.T) {
	lb := New("foo", func(ctx context.Context, request interface{}) string {
		return ""
	}, &mockBalancer{})
	assert.Equal(t, "tagging_mock", lb.Name())
}

func TestTaggingBalancer_GetPicker(t *testing.T) {
	testcases := []struct {
		cacheable    bool
		cacheKey     string
		instances    []discovery.Instance
		tagInstances map[string][]discovery.Instance
	}{
		{}, // nil
		{
			cacheable: true,
			cacheKey:  "one instance",
			instances: []discovery.Instance{discovery.NewInstance("tcp", "addr1", 10, map[string]string{"foo": "bar"})},
			tagInstances: map[string][]discovery.Instance{
				"bar": {discovery.NewInstance("tcp", "addr1", 10, map[string]string{"foo": "bar"})},
			},
		},
		{
			cacheable: false,
			cacheKey:  "multi instances",
			instances: []discovery.Instance{
				discovery.NewInstance("tcp", "addr1", 10, map[string]string{"foo": "bar1"}),
				discovery.NewInstance("tcp", "addr2", 20, map[string]string{"foo": "bar2"}),
				discovery.NewInstance("tcp", "addr3", 30, map[string]string{"foo": "bar3"}),
				discovery.NewInstance("tcp", "addr4", 30, map[string]string{"foo": ""}),
				discovery.NewInstance("tcp", "addr5", 30, nil),
			},
			tagInstances: map[string][]discovery.Instance{
				"bar1": {discovery.NewInstance("tcp", "addr1", 10, map[string]string{"foo": "bar1"})},
				"bar2": {discovery.NewInstance("tcp", "addr2", 20, map[string]string{"foo": "bar2"})},
				"bar3": {discovery.NewInstance("tcp", "addr3", 30, map[string]string{"foo": "bar3"})},
				"": {
					discovery.NewInstance("tcp", "addr4", 30, map[string]string{"foo": ""}),
					discovery.NewInstance("tcp", "addr5", 30, nil),
				},
			},
		},
	}

	lb := New("foo", func(ctx context.Context, request interface{}) string {
		return ""
	}, &mockBalancer{})

	for _, tt := range testcases {
		p := lb.GetPicker(discovery.Result{
			Cacheable: tt.cacheable,
			CacheKey:  tt.cacheKey,
			Instances: tt.instances,
		})
		assert.NotNil(t, p)
		assert.IsType(t, &taggingPicker{}, p)

		pp := p.(*taggingPicker)
		assert.Len(t, pp.tagPickers, len(tt.tagInstances))
		for k, v := range tt.tagInstances {
			p := pp.tagPickers[k]
			assert.IsType(t, &mockPicker{}, p)

			pp := p.(*mockPicker)
			assert.Equal(t, tt.cacheable, pp.result.Cacheable)
			assert.Equal(t, tt.cacheKey, pp.result.CacheKey)
			assert.EqualValues(t, v, pp.result.Instances)
		}
	}

	// once
	p1 := lb.GetPicker(discovery.Result{
		Cacheable: true,
		CacheKey:  "cached",
		Instances: nil,
	})

	p2 := lb.GetPicker(discovery.Result{
		Cacheable: true,
		CacheKey:  "cached",
		Instances: nil,
	})

	assert.Same(t, p1, p2)
}

func TestTaggingBalancer_Rebalance(t *testing.T) {
	lb := New("foo", func(ctx context.Context, request interface{}) string {
		return ""
	}, &mockBalancer{})

	p1 := lb.GetPicker(discovery.Result{
		Cacheable: true,
		CacheKey:  "rebalance",
		Instances: []discovery.Instance{discovery.NewInstance("tcp", "addr1", 10, map[string]string{"foo": "bar"})},
	})

	lb.(*taggingBalancer).Rebalance(discovery.Change{
		Result: discovery.Result{
			Cacheable: true,
			CacheKey:  "rebalance",
			Instances: []discovery.Instance{discovery.NewInstance("tcp", "addr2", 20, map[string]string{"foo": "bar"})},
		},
	})
	assert.True(t, lb.(*taggingBalancer).next.(*mockBalancer).deleted)

	p2 := lb.GetPicker(discovery.Result{
		Cacheable: true,
		CacheKey:  "rebalance",
		Instances: []discovery.Instance{discovery.NewInstance("tcp", "addr1", 10, map[string]string{"foo": "bar"})},
	})

	assert.NotEqual(t, p1, p2)

	mp1 := p1.(*taggingPicker).tagPickers["bar"].(*mockPicker)
	mp2 := p2.(*taggingPicker).tagPickers["bar"].(*mockPicker)
	assert.Equal(t, mp1.result, discovery.Result{
		Cacheable: true,
		CacheKey:  "rebalance",
		Instances: []discovery.Instance{discovery.NewInstance("tcp", "addr1", 10, map[string]string{"foo": "bar"})},
	})
	assert.Equal(t, mp2.result, discovery.Result{
		Cacheable: true,
		CacheKey:  "rebalance",
		Instances: []discovery.Instance{discovery.NewInstance("tcp", "addr2", 20, map[string]string{"foo": "bar"})},
	})
}

func TestTaggingBalancer_Delete(t *testing.T) {
	lb := New("foo", func(ctx context.Context, request interface{}) string {
		return ""
	}, &mockBalancer{})

	p := lb.GetPicker(discovery.Result{
		Cacheable: true,
		CacheKey:  "delete",
		Instances: []discovery.Instance{discovery.NewInstance("tcp", "addr1", 10, map[string]string{"foo": "bar"})},
	})

	pp, ok := lb.(*taggingBalancer).pickerCache.Load("delete")
	assert.True(t, ok)
	assert.Same(t, p, pp)

	lb.(*taggingBalancer).Delete(discovery.Change{
		Result: discovery.Result{
			Cacheable: true,
			CacheKey:  "delete",
		},
	})
	assert.True(t, lb.(*taggingBalancer).next.(*mockBalancer).deleted)

	pp, ok = lb.(*taggingBalancer).pickerCache.Load("delete")
	assert.False(t, ok)
	assert.Nil(t, pp)
}

func TestTaggingPicker_Next(t *testing.T) {
	lb := New("foo", func(ctx context.Context, request interface{}) string {
		return request.(string)
	}, &mockBalancer{})

	p := lb.GetPicker(discovery.Result{
		Instances: []discovery.Instance{
			discovery.NewInstance("tcp", "addr1", 10, map[string]string{"foo": "bar1"}),
			discovery.NewInstance("tcp", "addr2", 20, map[string]string{"foo": "bar2"}),
			discovery.NewInstance("tcp", "addr3", 30, map[string]string{"foo": "bar3"}),
			discovery.NewInstance("tcp", "addr4", 40, map[string]string{"foo": ""}),
		},
	})

	testcases := []struct {
		req    string
		expect discovery.Instance
	}{
		{"bar1", discovery.NewInstance("tcp", "addr1", 10, map[string]string{"foo": "bar1"})},
		{"bar2", discovery.NewInstance("tcp", "addr2", 20, map[string]string{"foo": "bar2"})},
		{"bar3", discovery.NewInstance("tcp", "addr3", 30, map[string]string{"foo": "bar3"})},
		{"", discovery.NewInstance("tcp", "addr4", 40, map[string]string{"foo": ""})},
		{"missed", nil},
	}

	for _, tt := range testcases {
		instance := p.Next(context.Background(), tt.req)
		assert.Equal(t, tt.expect, instance)
	}
}