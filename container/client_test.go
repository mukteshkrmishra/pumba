package container

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/docker/engine-api/types"
	"github.com/samalba/dockerclient/mockclient"
	"golang.org/x/net/context"

	"github.com/samalba/dockerclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func allContainers(Container) bool { return true }
func noContainers(Container) bool  { return false }

func TestListContainers_Success(t *testing.T) {
	ci := &dockerclient.ContainerInfo{Image: "abc123", Config: &dockerclient.ContainerConfig{Image: "img"}}
	ii := &dockerclient.ImageInfo{}
	api := mockclient.NewMockClient()
	api.On("ListContainers", false, false, "").Return([]dockerclient.Container{{Id: "foo", Names: []string{"bar"}}}, nil)
	api.On("InspectContainer", "foo").Return(ci, nil)
	api.On("InspectImage", "abc123").Return(ii, nil)

	client := dockerClient{api: api}
	cs, err := client.ListContainers(allContainers)

	assert.NoError(t, err)
	assert.Len(t, cs, 1)
	assert.Equal(t, ci, cs[0].containerInfo)
	assert.Equal(t, ii, cs[0].imageInfo)
	api.AssertExpectations(t)
}

func TestListContainers_Filter(t *testing.T) {
	ci := &dockerclient.ContainerInfo{Image: "abc123", Config: &dockerclient.ContainerConfig{Image: "img"}}
	ii := &dockerclient.ImageInfo{}
	api := mockclient.NewMockClient()
	api.On("ListContainers", false, false, "").Return([]dockerclient.Container{{Id: "foo", Names: []string{"bar"}}}, nil)
	api.On("InspectContainer", "foo").Return(ci, nil)
	api.On("InspectImage", "abc123").Return(ii, nil)

	client := dockerClient{api: api}
	cs, err := client.ListContainers(noContainers)

	assert.NoError(t, err)
	assert.Len(t, cs, 0)
	api.AssertExpectations(t)
}

func TestListContainers_ListError(t *testing.T) {
	api := mockclient.NewMockClient()
	api.On("ListContainers", false, false, "").Return([]dockerclient.Container{}, errors.New("oops"))

	client := dockerClient{api: api}
	_, err := client.ListContainers(allContainers)

	assert.Error(t, err)
	assert.EqualError(t, err, "oops")
	api.AssertExpectations(t)
}

func TestListContainers_InspectContainerError(t *testing.T) {
	api := mockclient.NewMockClient()
	api.On("ListContainers", false, false, "").Return([]dockerclient.Container{{Id: "foo", Names: []string{"bar"}}}, nil)
	api.On("InspectContainer", "foo").Return(&dockerclient.ContainerInfo{}, errors.New("uh-oh"))

	client := dockerClient{api: api}
	_, err := client.ListContainers(allContainers)

	assert.Error(t, err)
	assert.EqualError(t, err, "uh-oh")
	api.AssertExpectations(t)
}

func TestListContainers_InspectImageError(t *testing.T) {
	ci := &dockerclient.ContainerInfo{Image: "abc123", Config: &dockerclient.ContainerConfig{Image: "img"}}
	ii := &dockerclient.ImageInfo{}
	api := mockclient.NewMockClient()
	api.On("ListContainers", false, false, "").Return([]dockerclient.Container{{Id: "foo", Names: []string{"bar"}}}, nil)
	api.On("InspectContainer", "foo").Return(ci, nil)
	api.On("InspectImage", "abc123").Return(ii, errors.New("whoops"))

	client := dockerClient{api: api}
	_, err := client.ListContainers(allContainers)

	assert.Error(t, err)
	assert.EqualError(t, err, "whoops")
	api.AssertExpectations(t)
}

func TestStopContainer_DefaultSuccess(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:   "foo",
			Id:     "abc123",
			Config: &dockerclient.ContainerConfig{},
		},
	}

	ci := &dockerclient.ContainerInfo{
		State: &dockerclient.State{
			Running: false,
		},
	}

	api := mockclient.NewMockClient()
	api.On("KillContainer", "abc123", "SIGTERM").Return(nil)
	api.On("InspectContainer", "abc123").Return(ci, nil).Once()
	api.On("KillContainer", "abc123", "SIGKILL").Return(nil)
	api.On("InspectContainer", "abc123").Return(&dockerclient.ContainerInfo{}, errors.New("Not Found"))

	client := dockerClient{api: api}
	err := client.StopContainer(c, 1, false)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStopContainer_DryRun(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:   "foo",
			Id:     "abc123",
			Config: &dockerclient.ContainerConfig{},
		},
	}

	ci := &dockerclient.ContainerInfo{
		State: &dockerclient.State{
			Running: false,
		},
	}

	api := mockclient.NewMockClient()
	api.On("KillContainer", "abc123", "SIGTERM").Return(nil)
	api.On("InspectContainer", "abc123").Return(ci, nil).Once()
	api.On("KillContainer", "abc123", "SIGKILL").Return(nil)
	api.On("InspectContainer", "abc123").Return(&dockerclient.ContainerInfo{}, errors.New("Not Found"))

	client := dockerClient{api: api}
	err := client.StopContainer(c, 1, true)

	assert.NoError(t, err)
	api.AssertNotCalled(t, "KillContainer", "abc123", "SIGTERM")
	api.AssertNotCalled(t, "InspectContainer", "abc123")
	api.AssertNotCalled(t, "KillContainer", "abc123", "SIGKILL")
	api.AssertNotCalled(t, "InspectContainer", "abc123")
}

func TestKillContainer_DefaultSuccess(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:   "foo",
			Id:     "abc123",
			Config: &dockerclient.ContainerConfig{},
		},
	}

	api := mockclient.NewMockClient()
	api.On("KillContainer", "abc123", "SIGTERM").Return(nil)

	client := dockerClient{api: api}
	err := client.KillContainer(c, "SIGTERM", false)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestKillContainer_DryRun(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:   "foo",
			Id:     "abc123",
			Config: &dockerclient.ContainerConfig{},
		},
	}

	api := mockclient.NewMockClient()
	api.On("KillContainer", "abc123", "SIGTERM").Return(nil)

	client := dockerClient{api: api}
	err := client.KillContainer(c, "SIGTERM", true)

	assert.NoError(t, err)
	api.AssertNotCalled(t, "KillContainer", "abc123", "SIGTERM")
}

func TestStopContainer_CustomSignalSuccess(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name: "foo",
			Id:   "abc123",
			Config: &dockerclient.ContainerConfig{
				Labels: map[string]string{"com.gaiaadm.pumba.stop-signal": "SIGUSR1"}},
		},
	}

	ci := &dockerclient.ContainerInfo{
		State: &dockerclient.State{
			Running: false,
		},
	}

	api := mockclient.NewMockClient()
	api.On("KillContainer", "abc123", "SIGUSR1").Return(nil)
	api.On("InspectContainer", "abc123").Return(ci, nil).Once()
	api.On("KillContainer", "abc123", "SIGKILL").Return(nil)
	api.On("InspectContainer", "abc123").Return(&dockerclient.ContainerInfo{}, errors.New("Not Found"))

	client := dockerClient{api: api}
	err := client.StopContainer(c, 1, false)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStopContainer_KillContainerError(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:   "foo",
			Id:     "abc123",
			Config: &dockerclient.ContainerConfig{},
		},
	}

	api := mockclient.NewMockClient()
	api.On("KillContainer", "abc123", "SIGTERM").Return(errors.New("oops"))

	client := dockerClient{api: api}
	err := client.StopContainer(c, 1, false)

	assert.Error(t, err)
	assert.EqualError(t, err, "oops")
	api.AssertExpectations(t)
}

func TestStopContainer_2ndKillContainerError(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:   "foo",
			Id:     "abc123",
			Config: &dockerclient.ContainerConfig{},
		},
	}

	api := mockclient.NewMockClient()
	api.On("KillContainer", "abc123", "SIGTERM").Return(nil)
	api.On("InspectContainer", "abc123").Return(&dockerclient.ContainerInfo{}, errors.New("dangit"))
	api.On("KillContainer", "abc123", "SIGKILL").Return(errors.New("whoops"))

	client := dockerClient{api: api}
	err := client.StopContainer(c, 1, false)

	assert.Error(t, err)
	assert.EqualError(t, err, "whoops")
	api.AssertExpectations(t)
}

func TestStartContainer_Success(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:       "foo",
			Config:     &dockerclient.ContainerConfig{},
			HostConfig: &dockerclient.HostConfig{},
		},
		imageInfo: &dockerclient.ImageInfo{
			Config: &dockerclient.ContainerConfig{},
		},
	}

	api := mockclient.NewMockClient()
	api.On("CreateContainer",
		mock.AnythingOfType("*dockerclient.ContainerConfig"),
		"foo",
		mock.AnythingOfType("*dockerclient.AuthConfig")).Return("def789", nil)
	api.On("StartContainer", "def789", mock.AnythingOfType("*dockerclient.HostConfig")).Return(nil)

	client := dockerClient{api: api}
	err := client.StartContainer(c)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestStartContainer_CreateContainerError(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:       "foo",
			Config:     &dockerclient.ContainerConfig{},
			HostConfig: &dockerclient.HostConfig{},
		},
		imageInfo: &dockerclient.ImageInfo{
			Config: &dockerclient.ContainerConfig{},
		},
	}

	api := mockclient.NewMockClient()
	api.On("CreateContainer", mock.Anything, "foo", mock.Anything).Return("", errors.New("oops"))

	client := dockerClient{api: api}
	err := client.StartContainer(c)

	assert.Error(t, err)
	assert.EqualError(t, err, "oops")
	api.AssertExpectations(t)
}

func TestStartContainer_StartContainerError(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Name:       "foo",
			Config:     &dockerclient.ContainerConfig{},
			HostConfig: &dockerclient.HostConfig{},
		},
		imageInfo: &dockerclient.ImageInfo{
			Config: &dockerclient.ContainerConfig{},
		},
	}

	api := mockclient.NewMockClient()
	api.On("CreateContainer", mock.Anything, "foo", mock.Anything).Return("def789", nil)
	api.On("StartContainer", "def789", mock.Anything).Return(errors.New("whoops"))

	client := dockerClient{api: api}
	err := client.StartContainer(c)

	assert.Error(t, err)
	assert.EqualError(t, err, "whoops")
	api.AssertExpectations(t)
}

func TestRenameContainer_Success(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Id: "abc123",
		},
	}

	api := mockclient.NewMockClient()
	api.On("RenameContainer", "abc123", "foo").Return(nil)

	client := dockerClient{api: api}
	err := client.RenameContainer(c, "foo")

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestRenameContainer_Error(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Id: "abc123",
		},
	}

	api := mockclient.NewMockClient()
	api.On("RenameContainer", "abc123", "foo").Return(errors.New("oops"))

	client := dockerClient{api: api}
	err := client.RenameContainer(c, "foo")

	assert.Error(t, err)
	assert.EqualError(t, err, "oops")
	api.AssertExpectations(t)
}

func TestRemoveImage_Success(t *testing.T) {
	c := Container{
		imageInfo: &dockerclient.ImageInfo{
			Id: "abc123",
		},
	}

	api := mockclient.NewMockClient()
	api.On("RemoveImage", "abc123", false).Return([]*dockerclient.ImageDelete{}, nil)

	client := dockerClient{api: api}
	err := client.RemoveImage(c, false, false)

	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestRemoveImage_DryRun(t *testing.T) {
	c := Container{
		imageInfo: &dockerclient.ImageInfo{
			Id: "abc123",
		},
	}

	api := mockclient.NewMockClient()
	api.On("RemoveImage", "abc123", false).Return([]*dockerclient.ImageDelete{}, nil)

	client := dockerClient{api: api}
	err := client.RemoveImage(c, false, true)

	assert.NoError(t, err)
	api.AssertNotCalled(t, "RemoveImage", "abc123", false)
}

func TestRemoveContainer_Success(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Id: "abc123",
		},
	}

	ctx := context.Background()
	engineClient := NewMockEngine()
	removeOpts := types.ContainerRemoveOptions{RemoveVolumes: true, RemoveLinks: true, Force: true}
	engineClient.On("ContainerRemove", ctx, "abc123", removeOpts).Return(nil)

	client := dockerClient{apiClient: engineClient}
	err := client.RemoveContainer(c, true, true, true, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestRemoveContainer_DryRun(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Id: "abc123",
		},
	}

	ctx := context.Background()
	engineClient := NewMockEngine()
	removeOpts := types.ContainerRemoveOptions{RemoveVolumes: true, RemoveLinks: true, Force: true}
	engineClient.On("ContainerRemove", ctx, "abc123", removeOpts).Return(nil)

	client := dockerClient{apiClient: engineClient}
	err := client.RemoveContainer(c, true, true, true, true)

	assert.NoError(t, err)
	engineClient.AssertNotCalled(t, "ContainerRemove", ctx, "abc123", removeOpts)
}

func TestRemoveImage_Error(t *testing.T) {
	c := Container{
		imageInfo: &dockerclient.ImageInfo{
			Id: "abc123",
		},
	}

	api := mockclient.NewMockClient()
	api.On("RemoveImage", "abc123", false).Return([]*dockerclient.ImageDelete{}, errors.New("oops"))

	client := dockerClient{api: api}
	err := client.RemoveImage(c, false, false)

	assert.Error(t, err)
	assert.EqualError(t, err, "oops")
	api.AssertExpectations(t)
}

func TestPauseContainer_Success(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Id: "abc123",
		},
	}

	d, _ := time.ParseDuration("10ms")

	api := mockclient.NewMockClient()
	api.On("PauseContainer", "abc123").Return(nil)
	api.On("UnpauseContainer", "abc123").Return(nil)

	client := dockerClient{api: api}
	start := time.Now()
	err := client.PauseContainer(c, d, false)
	duration := time.Since(start)

	assert.True(t, duration >= d)
	assert.NoError(t, err)
	api.AssertExpectations(t)
}

func TestPauseContainer_DryRun(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Id: "abc123",
		},
	}

	d, _ := time.ParseDuration("2ms")

	api := mockclient.NewMockClient()

	client := dockerClient{api: api}
	err := client.PauseContainer(c, d, true)

	assert.NoError(t, err)
	api.AssertNotCalled(t, "PauseContainer", "abc123")
	api.AssertNotCalled(t, "UnpauseContainer", "abc123")
}

func TestPauseContainer_PauseError(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Id: "abc123",
		},
	}

	d, _ := time.ParseDuration("2ms")

	api := mockclient.NewMockClient()
	api.On("PauseContainer", "abc123").Return(errors.New("pause"))

	client := dockerClient{api: api}
	err := client.PauseContainer(c, d, false)

	assert.Error(t, err)
	assert.EqualError(t, err, "pause")
	api.AssertExpectations(t)
}

func TestPauseContainer_UnpauseError(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Id: "abc123",
		},
	}

	d, _ := time.ParseDuration("2ms")

	api := mockclient.NewMockClient()
	api.On("PauseContainer", "abc123").Return(nil)
	api.On("UnpauseContainer", "abc123").Return(errors.New("unpause"))

	client := dockerClient{api: api}
	err := client.PauseContainer(c, d, false)

	assert.Error(t, err)
	assert.EqualError(t, err, "unpause")
	api.AssertExpectations(t)
}

func TestNetemContainer_Success(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Id: "abc123",
		},
	}

	ctx := context.Background()
	engineClient := NewMockEngine()
	config := types.ExecConfig{Cmd: []string{"tc", "qdisc", "add", "dev", "eth0", "root", "netem", "delay", "1000ms"}, Privileged: true}
	engineClient.On("ContainerExecCreate", ctx, "abc123", config).Return(types.ContainerExecCreateResponse{"testID"}, nil)
	engineClient.On("ContainerExecStart", ctx, "testID", types.ExecStartCheck{}).Return(nil)
	stopConfig := types.ExecConfig{Cmd: []string{"tc", "qdisc", "del", "dev", "eth0", "root", "netem"}, Privileged: true}
	engineClient.On("ContainerExecCreate", ctx, "abc123", stopConfig).Return(types.ContainerExecCreateResponse{"testID"}, nil)
	engineClient.On("ContainerExecStart", ctx, "testID", types.ExecStartCheck{}).Return(nil)

	client := dockerClient{apiClient: engineClient}
	err := client.NetemContainer(c, "eth0", "delay 1000ms", nil, 1*time.Millisecond, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}

func TestNetemContainer_DryRun(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Id: "abc123",
		},
	}

	engineClient := NewMockEngine()
	client := dockerClient{apiClient: engineClient}
	err := client.NetemContainer(c, "eth0", "delay 1000ms", nil, 1*time.Millisecond, true)

	assert.NoError(t, err)
	engineClient.AssertNotCalled(t, "ContainerExecCreate", mock.Anything)
	engineClient.AssertNotCalled(t, "ContainerExecStart", "abc123", mock.Anything)
}

func TestNetemContainerIPFilter_Success(t *testing.T) {
	c := Container{
		containerInfo: &dockerclient.ContainerInfo{
			Id: "abc123",
		},
	}

	ctx := context.Background()
	engineClient := NewMockEngine()

	config1 := types.ExecConfig{Cmd: []string{"tc", "qdisc", "add", "dev", "eth0", "root", "handle", "1:", "prio"}, Privileged: true}
	engineClient.On("ContainerExecCreate", ctx, "abc123", config1).Return(types.ContainerExecCreateResponse{"cmd1"}, nil)
	engineClient.On("ContainerExecStart", ctx, "cmd1", types.ExecStartCheck{}).Return(nil)

	config2 := types.ExecConfig{Cmd: []string{"tc", "qdisc", "add", "dev", "eth0", "parent", "1:3", "netem", "delay", "1000ms"}, Privileged: true}
	engineClient.On("ContainerExecCreate", ctx, "abc123", config2).Return(types.ContainerExecCreateResponse{"cmd2"}, nil)
	engineClient.On("ContainerExecStart", ctx, "cmd2", types.ExecStartCheck{}).Return(nil)

	config3 := types.ExecConfig{Cmd: []string{"tc", "filter", "add", "dev", "eth0", "protocol", "ip",
		"parent", "1:0", "prio", "3", "u32", "match", "ip", "dport", "10.10.0.1", "flowid", "1:3"}, Privileged: true}
	engineClient.On("ContainerExecCreate", ctx, "abc123", config3).Return(types.ContainerExecCreateResponse{"cmd3"}, nil)
	engineClient.On("ContainerExecStart", ctx, "cmd3", types.ExecStartCheck{}).Return(nil)

	stopConfig := types.ExecConfig{Cmd: []string{"tc", "qdisc", "del", "dev", "eth0", "root", "netem"}, Privileged: true}
	engineClient.On("ContainerExecCreate", ctx, "abc123", stopConfig).Return(types.ContainerExecCreateResponse{"testID"}, nil)
	engineClient.On("ContainerExecStart", ctx, "testID", types.ExecStartCheck{}).Return(nil)

	client := dockerClient{apiClient: engineClient}
	err := client.NetemContainer(c, "eth0", "delay 1000ms", net.ParseIP("10.10.0.1"), 1*time.Millisecond, false)

	assert.NoError(t, err)
	engineClient.AssertExpectations(t)
}
