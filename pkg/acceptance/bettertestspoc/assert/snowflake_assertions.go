package assert

import (
	"errors"
	"fmt"
	"testing"

	"github.com/Snowflake-Labs/terraform-provider-snowflake/pkg/acceptance/helpers"
	"github.com/Snowflake-Labs/terraform-provider-snowflake/pkg/sdk"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/require"
)

type (
	assertSdk[T any]                                        func(*testing.T, T) error
	ObjectProvider[T any, I sdk.ObjectIdentifier]           func(*testing.T, I) (*T, error)
	testClientObjectProvider[T any, I sdk.ObjectIdentifier] func(client *helpers.TestClient) ObjectProvider[T, I]
)

// SnowflakeObjectAssert is an embeddable struct that should be used to construct new Snowflake object assertions.
// It implements both TestCheckFuncProvider and ImportStateCheckFuncProvider which makes it easy to create new resource assertions.
type SnowflakeObjectAssert[T any, I sdk.ObjectIdentifier] struct {
	assertions               []assertSdk[*T]
	id                       I
	objectType               sdk.ObjectType
	object                   *T
	provider                 ObjectProvider[T, I]
	testClientObjectProvider testClientObjectProvider[T, I]
}

// NewSnowflakeObjectAssertWithProvider creates a SnowflakeObjectAssert with id and the provider.
// Object to check is lazily fetched from Snowflake when the checks are being run.
func NewSnowflakeObjectAssertWithProvider[T any, I sdk.ObjectIdentifier](objectType sdk.ObjectType, id I, provider ObjectProvider[T, I]) *SnowflakeObjectAssert[T, I] {
	return &SnowflakeObjectAssert[T, I]{
		assertions: make([]assertSdk[*T], 0),
		id:         id,
		objectType: objectType,
		provider:   provider,
	}
}

// NewSnowflakeObjectAssertWithTestClientObjectProvider is temporary to show the new assertion setup with the test client.
func NewSnowflakeObjectAssertWithTestClientObjectProvider[T any, I sdk.ObjectIdentifier](objectType sdk.ObjectType, id I, testClientObjectProvider testClientObjectProvider[T, I]) *SnowflakeObjectAssert[T, I] {
	return &SnowflakeObjectAssert[T, I]{
		assertions:               make([]assertSdk[*T], 0),
		id:                       id,
		objectType:               objectType,
		testClientObjectProvider: testClientObjectProvider,
	}
}

// NewSnowflakeObjectAssertWithObject creates a SnowflakeObjectAssert with object that was already fetched from Snowflake.
// All the checks are run against the given object.
func NewSnowflakeObjectAssertWithObject[T any, I sdk.ObjectIdentifier](objectType sdk.ObjectType, id I, object *T) *SnowflakeObjectAssert[T, I] {
	return &SnowflakeObjectAssert[T, I]{
		assertions: make([]assertSdk[*T], 0),
		id:         id,
		objectType: objectType,
		object:     object,
	}
}

func (s *SnowflakeObjectAssert[T, I]) AddAssertion(assertion assertSdk[*T]) {
	s.assertions = append(s.assertions, assertion)
}

func (s *SnowflakeObjectAssert[T, I]) GetId() I {
	return s.id
}

// ToTerraformTestCheckFunc implements TestCheckFuncProvider to allow easier creation of new Snowflake object assertions.
// It goes through all the assertion accumulated earlier and gathers the results of the checks.
func (s *SnowflakeObjectAssert[_, _]) ToTerraformTestCheckFunc(t *testing.T) resource.TestCheckFunc {
	t.Helper()
	return func(_ *terraform.State) error {
		return s.runSnowflakeObjectsAssertions(t)
	}
}

// ToTerraformImportStateCheckFunc implements ImportStateCheckFuncProvider to allow easier creation of new Snowflake object assertions.
// It goes through all the assertion accumulated earlier and gathers the results of the checks.
func (s *SnowflakeObjectAssert[_, _]) ToTerraformImportStateCheckFunc(t *testing.T) resource.ImportStateCheckFunc {
	t.Helper()
	return func(_ []*terraform.InstanceState) error {
		return s.runSnowflakeObjectsAssertions(t)
	}
}

// VerifyAll implements InPlaceAssertionVerifier to allow easier creation of new Snowflake object assertions.
// It verifies all the assertions accumulated earlier and gathers the results of the checks.
func (s *SnowflakeObjectAssert[_, _]) VerifyAll(t *testing.T) {
	t.Helper()
	err := s.runSnowflakeObjectsAssertions(t)
	require.NoError(t, err)
}

// VerifyAllWithTestClient is temporary. It's here to show the changes proposed to the assertions setup.
func (s *SnowflakeObjectAssert[_, _]) VerifyAllWithTestClient(t *testing.T, testClient *helpers.TestClient) {
	t.Helper()
	err := s.runSnowflakeObjectsAssertionsWithTestClient(t, testClient)
	require.NoError(t, err)
}

func (s *SnowflakeObjectAssert[T, _]) runSnowflakeObjectsAssertions(t *testing.T) error {
	t.Helper()

	var sdkObject *T
	var err error
	switch {
	case s.object != nil:
		sdkObject = s.object
	case s.provider != nil:
		sdkObject, err = s.provider(t, s.id)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("cannot proceed with object %s[%s] assertion: object or provider must be specified", s.objectType, s.id.FullyQualifiedName())
	}

	var result []error

	for i, assertion := range s.assertions {
		if err = assertion(t, sdkObject); err != nil {
			result = append(result, fmt.Errorf("object %s[%s] assertion [%d/%d]: failed with error: %w", s.objectType, s.id.FullyQualifiedName(), i+1, len(s.assertions), err))
		}
	}

	return errors.Join(result...)
}

// runSnowflakeObjectsAssertionsWithTestClient is temporary until all assertion have the way to pass the test client used;
// should be renamed back to runSnowflakeObjectsAssertions when ready; its logic will be also adjusted.
func (s *SnowflakeObjectAssert[T, _]) runSnowflakeObjectsAssertionsWithTestClient(t *testing.T, testClient *helpers.TestClient) error {
	t.Helper()

	var sdkObject *T
	var err error
	switch {
	case s.object != nil:
		sdkObject = s.object
	case s.testClientObjectProvider != nil:
		if testClient == nil {
			return errors.New("testClient must not be nil")
		}
		sdkObject, err = s.testClientObjectProvider(testClient)(t, s.id)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("cannot proceed with object %s[%s] assertion: object or provider must be specified", s.objectType, s.id.FullyQualifiedName())
	}

	var result []error

	for i, assertion := range s.assertions {
		if err = assertion(t, sdkObject); err != nil {
			result = append(result, fmt.Errorf("object %s[%s] assertion [%d/%d]: failed with error: %w", s.objectType, s.id.FullyQualifiedName(), i+1, len(s.assertions), err))
		}
	}

	return errors.Join(result...)
}
