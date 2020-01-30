/*
Copyright © 2020 VMware

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by lister-gen. DO NOT EDIT.

package v1

import (
	v1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// HTTPProxyLister helps list HTTPProxies.
type HTTPProxyLister interface {
	// List lists all HTTPProxies in the indexer.
	List(selector labels.Selector) (ret []*v1.HTTPProxy, err error)
	// HTTPProxies returns an object that can list and get HTTPProxies.
	HTTPProxies(namespace string) HTTPProxyNamespaceLister
	HTTPProxyListerExpansion
}

// hTTPProxyLister implements the HTTPProxyLister interface.
type hTTPProxyLister struct {
	indexer cache.Indexer
}

// NewHTTPProxyLister returns a new HTTPProxyLister.
func NewHTTPProxyLister(indexer cache.Indexer) HTTPProxyLister {
	return &hTTPProxyLister{indexer: indexer}
}

// List lists all HTTPProxies in the indexer.
func (s *hTTPProxyLister) List(selector labels.Selector) (ret []*v1.HTTPProxy, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.HTTPProxy))
	})
	return ret, err
}

// HTTPProxies returns an object that can list and get HTTPProxies.
func (s *hTTPProxyLister) HTTPProxies(namespace string) HTTPProxyNamespaceLister {
	return hTTPProxyNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// HTTPProxyNamespaceLister helps list and get HTTPProxies.
type HTTPProxyNamespaceLister interface {
	// List lists all HTTPProxies in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1.HTTPProxy, err error)
	// Get retrieves the HTTPProxy from the indexer for a given namespace and name.
	Get(name string) (*v1.HTTPProxy, error)
	HTTPProxyNamespaceListerExpansion
}

// hTTPProxyNamespaceLister implements the HTTPProxyNamespaceLister
// interface.
type hTTPProxyNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all HTTPProxies in the indexer for a given namespace.
func (s hTTPProxyNamespaceLister) List(selector labels.Selector) (ret []*v1.HTTPProxy, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.HTTPProxy))
	})
	return ret, err
}

// Get retrieves the HTTPProxy from the indexer for a given namespace and name.
func (s hTTPProxyNamespaceLister) Get(name string) (*v1.HTTPProxy, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1.Resource("httpproxy"), name)
	}
	return obj.(*v1.HTTPProxy), nil
}
