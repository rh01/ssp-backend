package openshift

type Features struct {
	Nfs     bool `json:"nfs"`
	Gluster bool `json:"gluster"`
}

func GetFeatures(clusterId string) Features {
	cluster, _ := getOpenshiftCluster(clusterId)
	return Features{
		Gluster: cluster.GlusterApi != nil,
		Nfs:     cluster.NfsApi != nil,
	}
}
