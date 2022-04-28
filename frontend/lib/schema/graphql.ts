import { gql } from '@apollo/client'
import * as Apollo from '@apollo/client'
export type Maybe<T> = T | null
export type InputMaybe<T> = Maybe<T>
export type Exact<T extends { [key: string]: unknown }> = {
  [K in keyof T]: T[K]
}
export type MakeOptional<T, K extends keyof T> = Omit<T, K> & {
  [SubKey in K]?: Maybe<T[SubKey]>
}
export type MakeMaybe<T, K extends keyof T> = Omit<T, K> & {
  [SubKey in K]: Maybe<T[SubKey]>
}
const defaultOptions = {} as const
/** All built-in and custom scalars, mapped to their actual values */
export type Scalars = {
  ID: string
  String: string
  Boolean: boolean
  Int: number
  Float: number
  Map: any
  RawMessage: any
  /** Time is a string in [RFC 3339](https://rfc-editor.org/rfc/rfc3339.html) format, with sub-second precision added if present. */
  Time: any
}

export enum ConfigType {
  Bool = 'BOOL',
  Int = 'INT',
  String = 'STRING',
  StringArray = 'STRING_ARRAY',
}

export type Configuration = {
  __typename?: 'Configuration'
  created: Scalars['Time']
  description?: Maybe<Scalars['String']>
  env: Scalars['Boolean']
  environmentID?: Maybe<Scalars['ID']>
  feature: Scalars['String']
  id: Scalars['ID']
  key: Scalars['String']
  secret: Scalars['Boolean']
  type: ConfigType
  value: Scalars['RawMessage']
}

export type Environment = {
  __typename?: 'Environment'
  created: Scalars['Time']
  description?: Maybe<Scalars['String']>
  featureStates: Array<FeatureState>
  id: Scalars['ID']
  kind: EnvironmentKind
  lastModified: Scalars['Time']
  name: Scalars['String']
}

/** EnvironmentCreate contains metadata for creating an environment */
export type EnvironmentCreate = {
  description?: InputMaybe<Scalars['String']>
  kind: EnvironmentKind
  name: Scalars['String']
  tenantID: Scalars['ID']
}

export enum EnvironmentKind {
  Management = 'MANAGEMENT',
  Tenant = 'TENANT',
}

/** UpdateEnvironment contains metadata for updating an environment */
export type EnvironmentUpdate = {
  /** description of the environment */
  description?: InputMaybe<Scalars['String']>
}

export type Feature = {
  __typename?: 'Feature'
  chart: Scalars['String']
  config: Scalars['RawMessage']
  dependsOn: Array<Scalars['String']>
  environmentKinds: Array<EnvironmentKind>
  name: Scalars['String']
  repo: Scalars['String']
  source: Scalars['String']
  version: Scalars['String']
}

export type FeatureState = {
  __typename?: 'FeatureState'
  created?: Maybe<Scalars['Time']>
  enabled: Scalars['Boolean']
  feature: Feature
  lastModified?: Maybe<Scalars['Time']>
}

export type Mutation = {
  __typename?: 'Mutation'
  configurationCreate: Configuration
  configurationDelete: Scalars['Boolean']
  configurationUpdate: Configuration
  environmentCreate: Environment
  /** updateEnvironment updates an existing environment */
  environmentUpdate: Environment
  featureStateSave: FeatureState
  tenantCreate: Tenant
}

export type MutationConfigurationCreateArgs = {
  configuration: NewConfiguration
}

export type MutationConfigurationDeleteArgs = {
  id: Scalars['ID']
}

export type MutationConfigurationUpdateArgs = {
  configuration: UpdateConfiguration
  id: Scalars['ID']
}

export type MutationEnvironmentCreateArgs = {
  environment: EnvironmentCreate
}

export type MutationEnvironmentUpdateArgs = {
  id: Scalars['ID']
  input: EnvironmentUpdate
}

export type MutationFeatureStateSaveArgs = {
  enabled: Scalars['Boolean']
  envID: Scalars['ID']
  feature: Scalars['String']
}

export type MutationTenantCreateArgs = {
  tenant: TenantCreate
}

export type NewConfiguration = {
  description?: InputMaybe<Scalars['String']>
  environmentID?: InputMaybe<Scalars['ID']>
  feature: Scalars['String']
  key: Scalars['String']
  value: Scalars['RawMessage']
}

export type Query = {
  __typename?: 'Query'
  configuration: Array<Configuration>
  envConfig: Array<Configuration>
  /** Environment returns the given environment. */
  environment: Environment
  /** Environments returns the environments for a tenant. */
  environments: Array<Environment>
  features: Array<Feature>
  /** tenant returns the given tenant. */
  tenant: Tenant
  tenants: Array<Tenant>
  values: Scalars['Map']
}

export type QueryConfigurationArgs = {
  envID?: InputMaybe<Scalars['ID']>
  feature: Scalars['String']
}

export type QueryEnvConfigArgs = {
  envID: Scalars['ID']
  feature: Scalars['String']
}

export type QueryEnvironmentArgs = {
  id: Scalars['ID']
}

export type QueryEnvironmentsArgs = {
  tenantID: Scalars['ID']
}

export type QueryFeaturesArgs = {
  kind?: InputMaybe<EnvironmentKind>
}

export type QueryTenantArgs = {
  id: Scalars['ID']
}

export type QueryValuesArgs = {
  env: Scalars['ID']
  feature: Scalars['String']
}

export type Tenant = {
  __typename?: 'Tenant'
  created: Scalars['Time']
  description?: Maybe<Scalars['String']>
  id: Scalars['ID']
  lastModified: Scalars['Time']
  name: Scalars['String']
}

export type TenantCreate = {
  description?: InputMaybe<Scalars['String']>
  name: Scalars['String']
}

export type UpdateConfiguration = {
  description?: InputMaybe<Scalars['String']>
  value: Scalars['RawMessage']
}

export type ConfigGetQueryVariables = Exact<{
  feature: Scalars['String']
  envID: Scalars['ID']
}>

export type ConfigGetQuery = {
  __typename?: 'Query'
  envConfig: Array<{
    __typename?: 'Configuration'
    id: string
    description?: string | null
    value: any
    type: ConfigType
    env: boolean
    feature: string
    key: string
  }>
}

export type ConfigurationQueryVariables = Exact<{
  feature: Scalars['String']
  envID?: InputMaybe<Scalars['ID']>
}>

export type ConfigurationQuery = {
  __typename?: 'Query'
  configuration: Array<{
    __typename?: 'Configuration'
    id: string
    environmentID?: string | null
    feature: string
    description?: string | null
    key: string
    value: any
    secret: boolean
  }>
}

export type ConfigurationCreateMutationVariables = Exact<{
  description?: InputMaybe<Scalars['String']>
  feature: Scalars['String']
  key: Scalars['String']
  value: Scalars['RawMessage']
  environmentID?: InputMaybe<Scalars['ID']>
}>

export type ConfigurationCreateMutation = {
  __typename?: 'Mutation'
  configurationCreate: { __typename?: 'Configuration'; id: string; key: string }
}

export type ConfigurationDeleteMutationVariables = Exact<{
  id: Scalars['ID']
}>

export type ConfigurationDeleteMutation = {
  __typename?: 'Mutation'
  configurationDelete: boolean
}

export type ConfigurationUpdateMutationVariables = Exact<{
  description?: InputMaybe<Scalars['String']>
  id: Scalars['ID']
  value: Scalars['RawMessage']
}>

export type ConfigurationUpdateMutation = {
  __typename?: 'Mutation'
  configurationUpdate: { __typename?: 'Configuration'; id: string; key: string }
}

export type EnvironmentCreateMutationVariables = Exact<{
  name: Scalars['String']
  description?: InputMaybe<Scalars['String']>
  tenantID: Scalars['ID']
  kind: EnvironmentKind
}>

export type EnvironmentCreateMutation = {
  __typename?: 'Mutation'
  environmentCreate: { __typename?: 'Environment'; id: string }
}

export type EnvironmentGetQueryVariables = Exact<{
  id: Scalars['ID']
}>

export type EnvironmentGetQuery = {
  __typename?: 'Query'
  environment: {
    __typename?: 'Environment'
    id: string
    name: string
    description?: string | null
    lastModified: any
    created: any
    kind: EnvironmentKind
    featureStates: Array<{
      __typename?: 'FeatureState'
      enabled: boolean
      lastModified?: any | null
      created?: any | null
      feature: {
        __typename?: 'Feature'
        name: string
        version: string
        chart: string
        dependsOn: Array<string>
        repo: string
        source: string
        config: any
      }
    }>
  }
}

export type EnvironmentUpdateMutationVariables = Exact<{
  description?: InputMaybe<Scalars['String']>
  id: Scalars['ID']
}>

export type EnvironmentUpdateMutation = {
  __typename?: 'Mutation'
  environmentUpdate: { __typename?: 'Environment'; id: string }
}

export type EnvironmentsGetQueryVariables = Exact<{
  tenantID: Scalars['ID']
}>

export type EnvironmentsGetQuery = {
  __typename?: 'Query'
  environments: Array<{ __typename?: 'Environment'; id: string; name: string }>
}

export type FeaturesQueryVariables = Exact<{
  kind?: InputMaybe<EnvironmentKind>
}>

export type FeaturesQuery = {
  __typename?: 'Query'
  features: Array<{
    __typename?: 'Feature'
    dependsOn: Array<string>
    name: string
    chart: string
    config: any
    repo: string
    source: string
    environmentKinds: Array<EnvironmentKind>
    version: string
  }>
}

export type FeatureStateSaveMutationVariables = Exact<{
  envID: Scalars['ID']
  feature: Scalars['String']
  enabled: Scalars['Boolean']
}>

export type FeatureStateSaveMutation = {
  __typename?: 'Mutation'
  featureStateSave: { __typename?: 'FeatureState'; enabled: boolean }
}

export type TenantCreateMutationVariables = Exact<{
  name: Scalars['String']
  description?: InputMaybe<Scalars['String']>
}>

export type TenantCreateMutation = {
  __typename?: 'Mutation'
  tenantCreate: { __typename?: 'Tenant'; id: string }
}

export type TenantGetQueryVariables = Exact<{
  id: Scalars['ID']
}>

export type TenantGetQuery = {
  __typename?: 'Query'
  tenant: {
    __typename?: 'Tenant'
    id: string
    name: string
    description?: string | null
    created: any
    lastModified: any
  }
}

export type TenantsGetQueryVariables = Exact<{ [key: string]: never }>

export type TenantsGetQuery = {
  __typename?: 'Query'
  tenants: Array<{
    __typename?: 'Tenant'
    id: string
    name: string
    description?: string | null
    created: any
    lastModified: any
  }>
}

export const ConfigGetDocument = gql`
  query configGet($feature: String!, $envID: ID!) {
    envConfig(feature: $feature, envID: $envID) {
      id
      description
      value
      type
      env
      feature
      key
    }
  }
`

/**
 * __useConfigGetQuery__
 *
 * To run a query within a React component, call `useConfigGetQuery` and pass it any options that fit your needs.
 * When your component renders, `useConfigGetQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useConfigGetQuery({
 *   variables: {
 *      feature: // value for 'feature'
 *      envID: // value for 'envID'
 *   },
 * });
 */
export function useConfigGetQuery(
  baseOptions: Apollo.QueryHookOptions<ConfigGetQuery, ConfigGetQueryVariables>,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<ConfigGetQuery, ConfigGetQueryVariables>(
    ConfigGetDocument,
    options,
  )
}
export function useConfigGetLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    ConfigGetQuery,
    ConfigGetQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<ConfigGetQuery, ConfigGetQueryVariables>(
    ConfigGetDocument,
    options,
  )
}
export type ConfigGetQueryHookResult = ReturnType<typeof useConfigGetQuery>
export type ConfigGetLazyQueryHookResult = ReturnType<
  typeof useConfigGetLazyQuery
>
export type ConfigGetQueryResult = Apollo.QueryResult<
  ConfigGetQuery,
  ConfigGetQueryVariables
>
export const ConfigurationDocument = gql`
  query configuration($feature: String!, $envID: ID) {
    configuration(feature: $feature, envID: $envID) {
      id
      environmentID
      feature
      description
      key
      value
      secret
    }
  }
`

/**
 * __useConfigurationQuery__
 *
 * To run a query within a React component, call `useConfigurationQuery` and pass it any options that fit your needs.
 * When your component renders, `useConfigurationQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useConfigurationQuery({
 *   variables: {
 *      feature: // value for 'feature'
 *      envID: // value for 'envID'
 *   },
 * });
 */
export function useConfigurationQuery(
  baseOptions: Apollo.QueryHookOptions<
    ConfigurationQuery,
    ConfigurationQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<ConfigurationQuery, ConfigurationQueryVariables>(
    ConfigurationDocument,
    options,
  )
}
export function useConfigurationLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    ConfigurationQuery,
    ConfigurationQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<ConfigurationQuery, ConfigurationQueryVariables>(
    ConfigurationDocument,
    options,
  )
}
export type ConfigurationQueryHookResult = ReturnType<
  typeof useConfigurationQuery
>
export type ConfigurationLazyQueryHookResult = ReturnType<
  typeof useConfigurationLazyQuery
>
export type ConfigurationQueryResult = Apollo.QueryResult<
  ConfigurationQuery,
  ConfigurationQueryVariables
>
export const ConfigurationCreateDocument = gql`
  mutation configurationCreate(
    $description: String
    $feature: String!
    $key: String!
    $value: RawMessage!
    $environmentID: ID
  ) {
    configurationCreate(
      configuration: {
        feature: $feature
        description: $description
        key: $key
        value: $value
        environmentID: $environmentID
      }
    ) {
      id
      key
    }
  }
`
export type ConfigurationCreateMutationFn = Apollo.MutationFunction<
  ConfigurationCreateMutation,
  ConfigurationCreateMutationVariables
>

/**
 * __useConfigurationCreateMutation__
 *
 * To run a mutation, you first call `useConfigurationCreateMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useConfigurationCreateMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [configurationCreateMutation, { data, loading, error }] = useConfigurationCreateMutation({
 *   variables: {
 *      description: // value for 'description'
 *      feature: // value for 'feature'
 *      key: // value for 'key'
 *      value: // value for 'value'
 *      environmentID: // value for 'environmentID'
 *   },
 * });
 */
export function useConfigurationCreateMutation(
  baseOptions?: Apollo.MutationHookOptions<
    ConfigurationCreateMutation,
    ConfigurationCreateMutationVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useMutation<
    ConfigurationCreateMutation,
    ConfigurationCreateMutationVariables
  >(ConfigurationCreateDocument, options)
}
export type ConfigurationCreateMutationHookResult = ReturnType<
  typeof useConfigurationCreateMutation
>
export type ConfigurationCreateMutationResult =
  Apollo.MutationResult<ConfigurationCreateMutation>
export type ConfigurationCreateMutationOptions = Apollo.BaseMutationOptions<
  ConfigurationCreateMutation,
  ConfigurationCreateMutationVariables
>
export const ConfigurationDeleteDocument = gql`
  mutation configurationDelete($id: ID!) {
    configurationDelete(id: $id)
  }
`
export type ConfigurationDeleteMutationFn = Apollo.MutationFunction<
  ConfigurationDeleteMutation,
  ConfigurationDeleteMutationVariables
>

/**
 * __useConfigurationDeleteMutation__
 *
 * To run a mutation, you first call `useConfigurationDeleteMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useConfigurationDeleteMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [configurationDeleteMutation, { data, loading, error }] = useConfigurationDeleteMutation({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useConfigurationDeleteMutation(
  baseOptions?: Apollo.MutationHookOptions<
    ConfigurationDeleteMutation,
    ConfigurationDeleteMutationVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useMutation<
    ConfigurationDeleteMutation,
    ConfigurationDeleteMutationVariables
  >(ConfigurationDeleteDocument, options)
}
export type ConfigurationDeleteMutationHookResult = ReturnType<
  typeof useConfigurationDeleteMutation
>
export type ConfigurationDeleteMutationResult =
  Apollo.MutationResult<ConfigurationDeleteMutation>
export type ConfigurationDeleteMutationOptions = Apollo.BaseMutationOptions<
  ConfigurationDeleteMutation,
  ConfigurationDeleteMutationVariables
>
export const ConfigurationUpdateDocument = gql`
  mutation configurationUpdate(
    $description: String
    $id: ID!
    $value: RawMessage!
  ) {
    configurationUpdate(
      id: $id
      configuration: { description: $description, value: $value }
    ) {
      id
      key
    }
  }
`
export type ConfigurationUpdateMutationFn = Apollo.MutationFunction<
  ConfigurationUpdateMutation,
  ConfigurationUpdateMutationVariables
>

/**
 * __useConfigurationUpdateMutation__
 *
 * To run a mutation, you first call `useConfigurationUpdateMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useConfigurationUpdateMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [configurationUpdateMutation, { data, loading, error }] = useConfigurationUpdateMutation({
 *   variables: {
 *      description: // value for 'description'
 *      id: // value for 'id'
 *      value: // value for 'value'
 *   },
 * });
 */
export function useConfigurationUpdateMutation(
  baseOptions?: Apollo.MutationHookOptions<
    ConfigurationUpdateMutation,
    ConfigurationUpdateMutationVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useMutation<
    ConfigurationUpdateMutation,
    ConfigurationUpdateMutationVariables
  >(ConfigurationUpdateDocument, options)
}
export type ConfigurationUpdateMutationHookResult = ReturnType<
  typeof useConfigurationUpdateMutation
>
export type ConfigurationUpdateMutationResult =
  Apollo.MutationResult<ConfigurationUpdateMutation>
export type ConfigurationUpdateMutationOptions = Apollo.BaseMutationOptions<
  ConfigurationUpdateMutation,
  ConfigurationUpdateMutationVariables
>
export const EnvironmentCreateDocument = gql`
  mutation environmentCreate(
    $name: String!
    $description: String
    $tenantID: ID!
    $kind: EnvironmentKind!
  ) {
    environmentCreate(
      environment: {
        name: $name
        description: $description
        tenantID: $tenantID
        kind: $kind
      }
    ) {
      id
    }
  }
`
export type EnvironmentCreateMutationFn = Apollo.MutationFunction<
  EnvironmentCreateMutation,
  EnvironmentCreateMutationVariables
>

/**
 * __useEnvironmentCreateMutation__
 *
 * To run a mutation, you first call `useEnvironmentCreateMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useEnvironmentCreateMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [environmentCreateMutation, { data, loading, error }] = useEnvironmentCreateMutation({
 *   variables: {
 *      name: // value for 'name'
 *      description: // value for 'description'
 *      tenantID: // value for 'tenantID'
 *      kind: // value for 'kind'
 *   },
 * });
 */
export function useEnvironmentCreateMutation(
  baseOptions?: Apollo.MutationHookOptions<
    EnvironmentCreateMutation,
    EnvironmentCreateMutationVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useMutation<
    EnvironmentCreateMutation,
    EnvironmentCreateMutationVariables
  >(EnvironmentCreateDocument, options)
}
export type EnvironmentCreateMutationHookResult = ReturnType<
  typeof useEnvironmentCreateMutation
>
export type EnvironmentCreateMutationResult =
  Apollo.MutationResult<EnvironmentCreateMutation>
export type EnvironmentCreateMutationOptions = Apollo.BaseMutationOptions<
  EnvironmentCreateMutation,
  EnvironmentCreateMutationVariables
>
export const EnvironmentGetDocument = gql`
  query environmentGet($id: ID!) {
    environment(id: $id) {
      id
      name
      description
      lastModified
      created
      kind
      featureStates {
        enabled
        lastModified
        created
        feature {
          name
          version
          chart
          dependsOn
          repo
          source
          config
        }
      }
    }
  }
`

/**
 * __useEnvironmentGetQuery__
 *
 * To run a query within a React component, call `useEnvironmentGetQuery` and pass it any options that fit your needs.
 * When your component renders, `useEnvironmentGetQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useEnvironmentGetQuery({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useEnvironmentGetQuery(
  baseOptions: Apollo.QueryHookOptions<
    EnvironmentGetQuery,
    EnvironmentGetQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<EnvironmentGetQuery, EnvironmentGetQueryVariables>(
    EnvironmentGetDocument,
    options,
  )
}
export function useEnvironmentGetLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    EnvironmentGetQuery,
    EnvironmentGetQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<EnvironmentGetQuery, EnvironmentGetQueryVariables>(
    EnvironmentGetDocument,
    options,
  )
}
export type EnvironmentGetQueryHookResult = ReturnType<
  typeof useEnvironmentGetQuery
>
export type EnvironmentGetLazyQueryHookResult = ReturnType<
  typeof useEnvironmentGetLazyQuery
>
export type EnvironmentGetQueryResult = Apollo.QueryResult<
  EnvironmentGetQuery,
  EnvironmentGetQueryVariables
>
export const EnvironmentUpdateDocument = gql`
  mutation environmentUpdate($description: String, $id: ID!) {
    environmentUpdate(id: $id, input: { description: $description }) {
      id
    }
  }
`
export type EnvironmentUpdateMutationFn = Apollo.MutationFunction<
  EnvironmentUpdateMutation,
  EnvironmentUpdateMutationVariables
>

/**
 * __useEnvironmentUpdateMutation__
 *
 * To run a mutation, you first call `useEnvironmentUpdateMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useEnvironmentUpdateMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [environmentUpdateMutation, { data, loading, error }] = useEnvironmentUpdateMutation({
 *   variables: {
 *      description: // value for 'description'
 *      id: // value for 'id'
 *   },
 * });
 */
export function useEnvironmentUpdateMutation(
  baseOptions?: Apollo.MutationHookOptions<
    EnvironmentUpdateMutation,
    EnvironmentUpdateMutationVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useMutation<
    EnvironmentUpdateMutation,
    EnvironmentUpdateMutationVariables
  >(EnvironmentUpdateDocument, options)
}
export type EnvironmentUpdateMutationHookResult = ReturnType<
  typeof useEnvironmentUpdateMutation
>
export type EnvironmentUpdateMutationResult =
  Apollo.MutationResult<EnvironmentUpdateMutation>
export type EnvironmentUpdateMutationOptions = Apollo.BaseMutationOptions<
  EnvironmentUpdateMutation,
  EnvironmentUpdateMutationVariables
>
export const EnvironmentsGetDocument = gql`
  query environmentsGet($tenantID: ID!) {
    environments(tenantID: $tenantID) {
      id
      name
    }
  }
`

/**
 * __useEnvironmentsGetQuery__
 *
 * To run a query within a React component, call `useEnvironmentsGetQuery` and pass it any options that fit your needs.
 * When your component renders, `useEnvironmentsGetQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useEnvironmentsGetQuery({
 *   variables: {
 *      tenantID: // value for 'tenantID'
 *   },
 * });
 */
export function useEnvironmentsGetQuery(
  baseOptions: Apollo.QueryHookOptions<
    EnvironmentsGetQuery,
    EnvironmentsGetQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<EnvironmentsGetQuery, EnvironmentsGetQueryVariables>(
    EnvironmentsGetDocument,
    options,
  )
}
export function useEnvironmentsGetLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    EnvironmentsGetQuery,
    EnvironmentsGetQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<
    EnvironmentsGetQuery,
    EnvironmentsGetQueryVariables
  >(EnvironmentsGetDocument, options)
}
export type EnvironmentsGetQueryHookResult = ReturnType<
  typeof useEnvironmentsGetQuery
>
export type EnvironmentsGetLazyQueryHookResult = ReturnType<
  typeof useEnvironmentsGetLazyQuery
>
export type EnvironmentsGetQueryResult = Apollo.QueryResult<
  EnvironmentsGetQuery,
  EnvironmentsGetQueryVariables
>
export const FeaturesDocument = gql`
  query Features($kind: EnvironmentKind) {
    features(kind: $kind) {
      dependsOn
      name
      chart
      config
      repo
      source
      environmentKinds
      version
    }
  }
`

/**
 * __useFeaturesQuery__
 *
 * To run a query within a React component, call `useFeaturesQuery` and pass it any options that fit your needs.
 * When your component renders, `useFeaturesQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useFeaturesQuery({
 *   variables: {
 *      kind: // value for 'kind'
 *   },
 * });
 */
export function useFeaturesQuery(
  baseOptions?: Apollo.QueryHookOptions<FeaturesQuery, FeaturesQueryVariables>,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<FeaturesQuery, FeaturesQueryVariables>(
    FeaturesDocument,
    options,
  )
}
export function useFeaturesLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    FeaturesQuery,
    FeaturesQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<FeaturesQuery, FeaturesQueryVariables>(
    FeaturesDocument,
    options,
  )
}
export type FeaturesQueryHookResult = ReturnType<typeof useFeaturesQuery>
export type FeaturesLazyQueryHookResult = ReturnType<
  typeof useFeaturesLazyQuery
>
export type FeaturesQueryResult = Apollo.QueryResult<
  FeaturesQuery,
  FeaturesQueryVariables
>
export const FeatureStateSaveDocument = gql`
  mutation featureStateSave(
    $envID: ID!
    $feature: String!
    $enabled: Boolean!
  ) {
    featureStateSave(envID: $envID, feature: $feature, enabled: $enabled) {
      enabled
    }
  }
`
export type FeatureStateSaveMutationFn = Apollo.MutationFunction<
  FeatureStateSaveMutation,
  FeatureStateSaveMutationVariables
>

/**
 * __useFeatureStateSaveMutation__
 *
 * To run a mutation, you first call `useFeatureStateSaveMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useFeatureStateSaveMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [featureStateSaveMutation, { data, loading, error }] = useFeatureStateSaveMutation({
 *   variables: {
 *      envID: // value for 'envID'
 *      feature: // value for 'feature'
 *      enabled: // value for 'enabled'
 *   },
 * });
 */
export function useFeatureStateSaveMutation(
  baseOptions?: Apollo.MutationHookOptions<
    FeatureStateSaveMutation,
    FeatureStateSaveMutationVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useMutation<
    FeatureStateSaveMutation,
    FeatureStateSaveMutationVariables
  >(FeatureStateSaveDocument, options)
}
export type FeatureStateSaveMutationHookResult = ReturnType<
  typeof useFeatureStateSaveMutation
>
export type FeatureStateSaveMutationResult =
  Apollo.MutationResult<FeatureStateSaveMutation>
export type FeatureStateSaveMutationOptions = Apollo.BaseMutationOptions<
  FeatureStateSaveMutation,
  FeatureStateSaveMutationVariables
>
export const TenantCreateDocument = gql`
  mutation tenantCreate($name: String!, $description: String) {
    tenantCreate(tenant: { name: $name, description: $description }) {
      id
    }
  }
`
export type TenantCreateMutationFn = Apollo.MutationFunction<
  TenantCreateMutation,
  TenantCreateMutationVariables
>

/**
 * __useTenantCreateMutation__
 *
 * To run a mutation, you first call `useTenantCreateMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `useTenantCreateMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [tenantCreateMutation, { data, loading, error }] = useTenantCreateMutation({
 *   variables: {
 *      name: // value for 'name'
 *      description: // value for 'description'
 *   },
 * });
 */
export function useTenantCreateMutation(
  baseOptions?: Apollo.MutationHookOptions<
    TenantCreateMutation,
    TenantCreateMutationVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useMutation<
    TenantCreateMutation,
    TenantCreateMutationVariables
  >(TenantCreateDocument, options)
}
export type TenantCreateMutationHookResult = ReturnType<
  typeof useTenantCreateMutation
>
export type TenantCreateMutationResult =
  Apollo.MutationResult<TenantCreateMutation>
export type TenantCreateMutationOptions = Apollo.BaseMutationOptions<
  TenantCreateMutation,
  TenantCreateMutationVariables
>
export const TenantGetDocument = gql`
  query TenantGet($id: ID!) {
    tenant(id: $id) {
      id
      name
      description
      created
      lastModified
    }
  }
`

/**
 * __useTenantGetQuery__
 *
 * To run a query within a React component, call `useTenantGetQuery` and pass it any options that fit your needs.
 * When your component renders, `useTenantGetQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useTenantGetQuery({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function useTenantGetQuery(
  baseOptions: Apollo.QueryHookOptions<TenantGetQuery, TenantGetQueryVariables>,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<TenantGetQuery, TenantGetQueryVariables>(
    TenantGetDocument,
    options,
  )
}
export function useTenantGetLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    TenantGetQuery,
    TenantGetQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<TenantGetQuery, TenantGetQueryVariables>(
    TenantGetDocument,
    options,
  )
}
export type TenantGetQueryHookResult = ReturnType<typeof useTenantGetQuery>
export type TenantGetLazyQueryHookResult = ReturnType<
  typeof useTenantGetLazyQuery
>
export type TenantGetQueryResult = Apollo.QueryResult<
  TenantGetQuery,
  TenantGetQueryVariables
>
export const TenantsGetDocument = gql`
  query TenantsGet {
    tenants {
      id
      name
      description
      created
      lastModified
    }
  }
`

/**
 * __useTenantsGetQuery__
 *
 * To run a query within a React component, call `useTenantsGetQuery` and pass it any options that fit your needs.
 * When your component renders, `useTenantsGetQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = useTenantsGetQuery({
 *   variables: {
 *   },
 * });
 */
export function useTenantsGetQuery(
  baseOptions?: Apollo.QueryHookOptions<
    TenantsGetQuery,
    TenantsGetQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<TenantsGetQuery, TenantsGetQueryVariables>(
    TenantsGetDocument,
    options,
  )
}
export function useTenantsGetLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    TenantsGetQuery,
    TenantsGetQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<TenantsGetQuery, TenantsGetQueryVariables>(
    TenantsGetDocument,
    options,
  )
}
export type TenantsGetQueryHookResult = ReturnType<typeof useTenantsGetQuery>
export type TenantsGetLazyQueryHookResult = ReturnType<
  typeof useTenantsGetLazyQuery
>
export type TenantsGetQueryResult = Apollo.QueryResult<
  TenantsGetQuery,
  TenantsGetQueryVariables
>
