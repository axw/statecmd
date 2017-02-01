package main

import (
	"log"
	"os"

	"github.com/juju/errors"
	"github.com/juju/gnuflag"
	"github.com/juju/juju/agent"
	jujudagent "github.com/juju/juju/cmd/jujud/agent"
	"github.com/juju/juju/environs"
	"github.com/juju/juju/mongo"
	"github.com/juju/juju/state"
	"github.com/juju/juju/state/stateenvirons"
	"github.com/juju/replicaset"
	"github.com/juju/utils/clock"
	"gopkg.in/juju/names.v2"
	mgo "gopkg.in/mgo.v2"
)

func Main() error {
	agentConf, err := initAgentConf()
	if err != nil {
		return err
	}

	machineTag, err := identifyMachineAgent(agentConf.DataDir())
	if err != nil {
		return err
	}

	if err := agentConf.ReadConfig(machineTag.String()); err != nil {
		return err
	}

	st, err := openState(agentConf.CurrentConfig())
	if err != nil {
		return err
	}
	defer st.Close()

	// DO STUFF WITH STATE HERE

	return nil
}

func initAgentConf() (jujudagent.AgentConf, error) {
	flags := gnuflag.NewFlagSet("juju", gnuflag.ExitOnError)
	conf := jujudagent.NewAgentConf("/var/lib/juju")
	conf.AddFlags(flags)
	return conf, flags.Parse(true, os.Args)
}

func identifyMachineAgent(dataDir string) (names.MachineTag, error) {
	f, err := os.Open(agent.BaseDir(dataDir))
	if err != nil {
		return names.MachineTag{}, err
	}
	defer f.Close()

	ents, err := f.Readdir(-1)
	if err != nil {
		return names.MachineTag{}, err
	}
	for _, ent := range ents {
		tag, err := names.ParseMachineTag(ent.Name())
		if err == nil {
			return tag, nil
		}
	}
	return names.MachineTag{}, errors.NotFoundf("machine agent")
}

func openState(agentConfig agent.Config) (*state.State, error) {
	mongoInfo, ok := agentConfig.MongoInfo()
	if !ok {
		return nil, errors.New("no mongo info available")
	}
	mongoDialOpts := mongo.DefaultDialOpts()
	mongoDialOpts.PostDial = func(session *mgo.Session) error {
		safe := mgo.Safe{J: true}
		if _, err := replicaset.CurrentConfig(session); err == nil {
			safe.WMode = "majority"
		}
		session.SetSafe(&safe)
		return nil
	}
	return state.Open(state.OpenParams{
		Clock:              clock.WallClock,
		ControllerTag:      agentConfig.Controller(),
		ControllerModelTag: agentConfig.Model(),
		MongoInfo:          mongoInfo,
		MongoDialOpts:      mongoDialOpts,
		NewPolicy: stateenvirons.GetNewPolicyFunc(
			stateenvirons.GetNewEnvironFunc(environs.New),
		),
	})
}

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}
