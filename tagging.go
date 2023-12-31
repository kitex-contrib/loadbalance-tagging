/*
 * Copyright 2023 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package tagging

import (
	"context"
	"sync"

	"golang.org/x/sync/singleflight"

	"github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/loadbalance"
)

type TagFunc func(ctx context.Context, request interface{}) string

type taggingBalancer struct {
	tag         string
	tagFunc     TagFunc
	next        loadbalance.Loadbalancer
	pickerCache sync.Map
	sfg         singleflight.Group
}

func New(tag string, f TagFunc, next loadbalance.Loadbalancer) loadbalance.Loadbalancer {
	return &taggingBalancer{
		tag:     tag,
		tagFunc: f,
		next:    next,
	}
}

func (b *taggingBalancer) GetPicker(e discovery.Result) loadbalance.Picker {
	if !e.Cacheable {
		p := b.createPicker(e)
		return p
	}

	p, ok := b.pickerCache.Load(e.CacheKey)
	if !ok {
		p, _, _ = b.sfg.Do(e.CacheKey, func() (interface{}, error) {
			pp := b.createPicker(e)
			b.pickerCache.Store(e.CacheKey, pp)
			return pp, nil
		})
	}
	return p.(loadbalance.Picker)
}

func (b *taggingBalancer) Rebalance(change discovery.Change) {
	if !change.Result.Cacheable {
		return
	}
	b.pickerCache.Store(change.Result.CacheKey, b.createPicker(change.Result))
}

func (b *taggingBalancer) Delete(change discovery.Change) {
	if !change.Result.Cacheable {
		return
	}
	b.pickerCache.Delete(change.Result.CacheKey)
}

func (b *taggingBalancer) createPicker(e discovery.Result) loadbalance.Picker {
	instances := make(map[string][]discovery.Instance)
	for _, instance := range e.Instances {
		if t, ok := instance.Tag(b.tag); ok {
			instances[t] = append(instances[t], instance)
		} else {
			instances[""] = append(instances[""], instance)
		}
	}

	pickers := make(map[string]loadbalance.Picker, len(instances))
	for t, instances := range instances {
		p := b.next.GetPicker(discovery.Result{
			Instances: instances,
		})
		pickers[t] = p
	}

	return &taggingPicker{
		tagFunc:    b.tagFunc,
		tagPickers: pickers,
	}
}

func (b *taggingBalancer) Name() string {
	return "tagging_" + b.next.Name()
}

type taggingPicker struct {
	tagFunc    TagFunc
	tagPickers map[string]loadbalance.Picker
}

func (p *taggingPicker) Next(ctx context.Context, request interface{}) discovery.Instance {
	t := p.tagFunc(ctx, request)
	if pp, ok := p.tagPickers[t]; ok {
		return pp.Next(ctx, request)
	}
	return nil
}
