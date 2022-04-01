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
  lastModified: Scalars['Time']
  name: Scalars['String']
}

/** EnvironmentCreate contains metadata for creating an environment */
export type EnvironmentCreate = {
  description?: InputMaybe<Scalars['String']>
  name: Scalars['String']
  partnerID: Scalars['ID']
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
  partnerCreate: Partner
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

export type MutationPartnerCreateArgs = {
  partner: PartnerCreate
}

export type NewConfiguration = {
  description?: InputMaybe<Scalars['String']>
  environmentID?: InputMaybe<Scalars['ID']>
  feature: Scalars['String']
  key: Scalars['String']
  value: Scalars['RawMessage']
}

export type Partner = {
  __typename?: 'Partner'
  created: Scalars['Time']
  description?: Maybe<Scalars['String']>
  id: Scalars['ID']
  lastModified: Scalars['Time']
  name: Scalars['String']
}

export type PartnerCreate = {
  description?: InputMaybe<Scalars['String']>
  name: Scalars['String']
}

export type Query = {
  __typename?: 'Query'
  configuration: Array<Configuration>
  envConfig: Array<Configuration>
  /** Environment returns the given environment. */
  environment: Environment
  /** Environments returns the environments for a partner. */
  environments: Array<Environment>
  features: Array<Feature>
  /** partner returns the given partner. */
  partner: Partner
  partners: Array<Partner>
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
  partnerID: Scalars['ID']
}

export type QueryPartnerArgs = {
  id: Scalars['ID']
}

export type QueryValuesArgs = {
  env: Scalars['ID']
  feature: Scalars['String']
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
  partnerID: Scalars['ID']
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
  partnerID: Scalars['ID']
}>

export type EnvironmentsGetQuery = {
  __typename?: 'Query'
  environments: Array<{ __typename?: 'Environment'; id: string; name: string }>
}

export type FeaturesQueryVariables = Exact<{ [key: string]: never }>

export type FeaturesQuery = {
  __typename?: 'Query'
  features: Array<{
    __typename?: 'Feature'
    name: string
    chart: string
    config: any
    repo: string
    source: string
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

export type PartnerCreateMutationVariables = Exact<{
  name: Scalars['String']
  description?: InputMaybe<Scalars['String']>
}>

export type PartnerCreateMutation = {
  __typename?: 'Mutation'
  partnerCreate: { __typename?: 'Partner'; id: string }
}

export type PartnerGetQueryVariables = Exact<{
  id: Scalars['ID']
}>

export type PartnerGetQuery = {
  __typename?: 'Query'
  partner: {
    __typename?: 'Partner'
    id: string
    name: string
    description?: string | null
    created: any
    lastModified: any
  }
}

export type PartnersGetQueryVariables = Exact<{ [key: string]: never }>

export type PartnersGetQuery = {
  __typename?: 'Query'
  partners: Array<{
    __typename?: 'Partner'
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
    $partnerID: ID!
  ) {
    environmentCreate(
      environment: {
        name: $name
        description: $description
        partnerID: $partnerID
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
 *      partnerID: // value for 'partnerID'
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
      featureStates {
        enabled
        lastModified
        created
        feature {
          name
          version
          chart
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
  query environmentsGet($partnerID: ID!) {
    environments(partnerID: $partnerID) {
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
 *      partnerID: // value for 'partnerID'
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
  query Features {
    features {
      name
      chart
      config
      repo
      source
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
export const PartnerCreateDocument = gql`
  mutation partnerCreate($name: String!, $description: String) {
    partnerCreate(partner: { name: $name, description: $description }) {
      id
    }
  }
`
export type PartnerCreateMutationFn = Apollo.MutationFunction<
  PartnerCreateMutation,
  PartnerCreateMutationVariables
>

/**
 * __usePartnerCreateMutation__
 *
 * To run a mutation, you first call `usePartnerCreateMutation` within a React component and pass it any options that fit your needs.
 * When your component renders, `usePartnerCreateMutation` returns a tuple that includes:
 * - A mutate function that you can call at any time to execute the mutation
 * - An object with fields that represent the current status of the mutation's execution
 *
 * @param baseOptions options that will be passed into the mutation, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options-2;
 *
 * @example
 * const [partnerCreateMutation, { data, loading, error }] = usePartnerCreateMutation({
 *   variables: {
 *      name: // value for 'name'
 *      description: // value for 'description'
 *   },
 * });
 */
export function usePartnerCreateMutation(
  baseOptions?: Apollo.MutationHookOptions<
    PartnerCreateMutation,
    PartnerCreateMutationVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useMutation<
    PartnerCreateMutation,
    PartnerCreateMutationVariables
  >(PartnerCreateDocument, options)
}
export type PartnerCreateMutationHookResult = ReturnType<
  typeof usePartnerCreateMutation
>
export type PartnerCreateMutationResult =
  Apollo.MutationResult<PartnerCreateMutation>
export type PartnerCreateMutationOptions = Apollo.BaseMutationOptions<
  PartnerCreateMutation,
  PartnerCreateMutationVariables
>
export const PartnerGetDocument = gql`
  query PartnerGet($id: ID!) {
    partner(id: $id) {
      id
      name
      description
      created
      lastModified
    }
  }
`

/**
 * __usePartnerGetQuery__
 *
 * To run a query within a React component, call `usePartnerGetQuery` and pass it any options that fit your needs.
 * When your component renders, `usePartnerGetQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = usePartnerGetQuery({
 *   variables: {
 *      id: // value for 'id'
 *   },
 * });
 */
export function usePartnerGetQuery(
  baseOptions: Apollo.QueryHookOptions<
    PartnerGetQuery,
    PartnerGetQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<PartnerGetQuery, PartnerGetQueryVariables>(
    PartnerGetDocument,
    options,
  )
}
export function usePartnerGetLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    PartnerGetQuery,
    PartnerGetQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<PartnerGetQuery, PartnerGetQueryVariables>(
    PartnerGetDocument,
    options,
  )
}
export type PartnerGetQueryHookResult = ReturnType<typeof usePartnerGetQuery>
export type PartnerGetLazyQueryHookResult = ReturnType<
  typeof usePartnerGetLazyQuery
>
export type PartnerGetQueryResult = Apollo.QueryResult<
  PartnerGetQuery,
  PartnerGetQueryVariables
>
export const PartnersGetDocument = gql`
  query PartnersGet {
    partners {
      id
      name
      description
      created
      lastModified
    }
  }
`

/**
 * __usePartnersGetQuery__
 *
 * To run a query within a React component, call `usePartnersGetQuery` and pass it any options that fit your needs.
 * When your component renders, `usePartnersGetQuery` returns an object from Apollo Client that contains loading, error, and data properties
 * you can use to render your UI.
 *
 * @param baseOptions options that will be passed into the query, supported options are listed on: https://www.apollographql.com/docs/react/api/react-hooks/#options;
 *
 * @example
 * const { data, loading, error } = usePartnersGetQuery({
 *   variables: {
 *   },
 * });
 */
export function usePartnersGetQuery(
  baseOptions?: Apollo.QueryHookOptions<
    PartnersGetQuery,
    PartnersGetQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useQuery<PartnersGetQuery, PartnersGetQueryVariables>(
    PartnersGetDocument,
    options,
  )
}
export function usePartnersGetLazyQuery(
  baseOptions?: Apollo.LazyQueryHookOptions<
    PartnersGetQuery,
    PartnersGetQueryVariables
  >,
) {
  const options = { ...defaultOptions, ...baseOptions }
  return Apollo.useLazyQuery<PartnersGetQuery, PartnersGetQueryVariables>(
    PartnersGetDocument,
    options,
  )
}
export type PartnersGetQueryHookResult = ReturnType<typeof usePartnersGetQuery>
export type PartnersGetLazyQueryHookResult = ReturnType<
  typeof usePartnersGetLazyQuery
>
export type PartnersGetQueryResult = Apollo.QueryResult<
  PartnersGetQuery,
  PartnersGetQueryVariables
>
