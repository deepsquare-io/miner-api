//go:build unit

package scheduler_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/squarefactory/miner-api/mocks"
	"github.com/squarefactory/miner-api/scheduler"
	"github.com/squarefactory/miner-api/utils"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

var (
	user  = "fakeUser"
	admin = "fakeAdmin"
	pkB64 = "private key"
)

type ServiceTestSuite struct {
	suite.Suite
	executor *mocks.Executor
	impl     *scheduler.Slurm
}

func (suite *ServiceTestSuite) BeforeTest(suiteName, testName string) {
	suite.executor = mocks.NewExecutor(suite.T())
	suite.impl = scheduler.NewSlurm(
		suite.executor,
		admin,
	)
}

func (suite *ServiceTestSuite) TestCancel() {
	// Arrange
	name := utils.GenerateRandomString(6)
	req := &scheduler.CancelRequest{
		Name: name,
		User: user,
	}
	suite.executor.On(
		"ExecAs",
		mock.Anything,
		user,
		mock.MatchedBy(func(cmd string) bool {
			return strings.Contains(cmd, "scancel") &&
				strings.Contains(cmd, req.Name)
		}),
	).Return("ok", nil)
	ctx := context.Background()

	// Act
	err := suite.impl.CancelJob(ctx, req)

	// Assert
	suite.NoError(err)
	suite.executor.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestSubmit() {
	// Arrange
	name := utils.GenerateRandomString(6)
	expectedJobID := "123"
	req := &scheduler.SubmitRequest{
		Name: name,
		User: user,
		Body: `#!/bin/sh

srun sleep infinity
`,
	}
	suite.executor.On(
		"ExecAs",
		mock.Anything,
		user,
		mock.MatchedBy(func(cmd string) bool {
			return strings.Contains(cmd, "sbatch") &&
				strings.Contains(cmd, req.Name) &&
				strings.Contains(cmd, req.Body)
		}),
	).Return(fmt.Sprintf("%s\n", expectedJobID), nil)
	ctx := context.Background()

	// Act
	jobID, err := suite.impl.Submit(ctx, req)

	// Assert
	suite.NoError(err)
	suite.Equal(expectedJobID, jobID)
	suite.executor.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestHealthCheck() {
	// Arrange
	suite.executor.On(
		"ExecAs",
		mock.Anything,
		admin,
		"squeue",
	).Return("ok", nil)
	ctx := context.Background()

	// Act
	err := suite.impl.HealthCheck(ctx)

	// Assert
	suite.NoError(err)
	suite.executor.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestFindRunningJobByName() {
	// Arrange
	name := utils.GenerateRandomString(6)
	jobID := 123
	req := &scheduler.FindRunningJobByNameRequest{
		Name: name,
		User: user,
	}
	suite.executor.On(
		"ExecAs",
		mock.Anything,
		user,
		mock.MatchedBy(func(cmd string) bool {
			return strings.Contains(cmd, "squeue") &&
				strings.Contains(cmd, name)
		}),
	).Return(fmt.Sprintf("%d\n", jobID), nil)
	ctx := context.Background()

	// Act
	out, err := suite.impl.FindRunningJobByName(ctx, req)

	// Assert
	suite.NoError(err)
	suite.Equal(jobID, out)
	suite.executor.AssertExpectations(suite.T())
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, &ServiceTestSuite{})
}
