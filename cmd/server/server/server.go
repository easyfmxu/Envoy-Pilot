package server

import (
	"Envoy-xDS/cmd/server/model"
	"Envoy-xDS/cmd/server/service"
	"Envoy-xDS/cmd/server/storage"
	"context"
	"log"
	"strings"
)

const envoySubscriberKey = "envoySubscriber"

var defaultPushService *service.DefaultPushService
var xdsConfigDao *storage.XdsConfigDao

func init() {
	defaultPushService = service.GetDefaultPushService()
	xdsConfigDao = storage.GetXdsConfigDao()
}

func getReqVersion(version string) string {
	if len(version) != 0 {
		return strings.Trim(version, `"'`)
	}
	return ""
}

// Server struct will impl CDS, LDS, RDS & ADS
type Server struct{}

// BiDiStreamFor common bi-directional stream impl for cds,lds,rds
func (s *Server) BiDiStreamFor(xdsType string, stream service.XDSStreamServer) error {
	log.Printf("-------------- Starting a %s stream ------------------\n", xdsType)

	serverCtx, cancel := context.WithCancel(context.Background())
	dispatchChannel := make(chan string)
	i := 0
	var subscriber *model.EnvoySubscriber

	for {
		req, err := stream.Recv()
		if err != nil {
			log.Printf("Disconnecting client %s\n", subscriber.BuildInstanceKey())
			log.Println(err)
			cancel()
			return err
		}
		if i == 0 {
			subscriber = &model.EnvoySubscriber{
				Cluster:            req.Node.Cluster,
				Node:               req.Node.Id,
				SubscribedTo:       xdsType,
				LastUpdatedVersion: getReqVersion(req.VersionInfo),
			}
			serverCtx = context.WithValue(serverCtx, envoySubscriberKey, subscriber)
			defaultPushService.RegisterEnvoy(serverCtx, stream, subscriber, dispatchChannel)
			i++
		}

		log.Printf("Received Request from %s\n %+v\n", subscriber.BuildInstanceKey(), req)

		if xdsConfigDao.IsACK(subscriber, req.ResponseNonce) {
			defaultPushService.HandleACK(subscriber, req)
			continue
		} else {
			log.Printf("Response nonce not recognized %s", req.ResponseNonce)
		}
	}
}
