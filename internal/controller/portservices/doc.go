// Package portservices contains pure helpers for merging effective per-environment static services
// with the controller root service pool (config + K8s globals). The same policy is used by
// the xDS data plane and the API Server registry payload so behavior stays aligned.
package portservices
