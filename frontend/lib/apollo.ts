import { useMemo } from 'react'
import { ApolloClient, HttpLink, InMemoryCache, NormalizedCacheObject } from '@apollo/client'
import merge from 'deepmerge'
import isEqual from 'lodash.isequal'

const isServer = typeof window === 'undefined'

export const APOLLO_STATE_PROP_NAME = '__APOLLO_STATE__'

let apolloClient: ApolloClient<NormalizedCacheObject> | undefined

const getQueryURI = () => {
    if (process.env.NEXT_PUBLIC_ENV === 'development') {
        return 'http://localhost:8080/query'
    }

    return isServer ? 'http://fasit-backend/query' : 'http://localhost:8080/query'
}

const createApolloClient = (cookie?: string) => {
    return new ApolloClient({
        ssrMode: typeof window === 'undefined',
        link: new HttpLink({
            uri: getQueryURI(),
            credentials: 'include',
            headers: {
                cookie,
            }
        }),
        cache: new InMemoryCache({
                typePolicies: {
                    Configuration: {
                        keyFields: [["key"], ["id"]],
                    },
                }
            }
        ),
    })
}

export const initializeApollo = (initialState: NormalizedCacheObject | null = null, cookie?: string) => {
    const _apolloClient = apolloClient ?? createApolloClient(cookie)

    // If your page has Next.js data fetching methods that use Apollo Client, the initial state
    // gets hydrated here
    if (initialState) {
        // Get existing cache, loaded during client side data fetching
        const existingCache = _apolloClient.extract()

        // Merge the existing cache into data passed from getStaticProps/getServerSideProps
        const data = merge(initialState, existingCache, {
            // combine arrays using object equality (like in sets)
            arrayMerge: (destinationArray, sourceArray) => [
                ...sourceArray,
                ...destinationArray.filter((d) =>
                    sourceArray.every((s) => !isEqual(d, s)),
                ),
            ],
        })

        // Restore the cache with the merged data
        _apolloClient.cache.restore(data)
    }

    // For SSG and SSR always create a new Apollo Client
    if (isServer) return _apolloClient

    // Create the Apollo Client once in the client
    if (!apolloClient) {
        apolloClient = _apolloClient
    }

    return _apolloClient
}

export const addApolloState = (client: ApolloClient<NormalizedCacheObject>, pageProps: any) => {
    if (pageProps?.props) {
        pageProps.props[APOLLO_STATE_PROP_NAME] = client.cache.extract();
    }

    return pageProps;
}

export const useApollo = (pageProps: any) => {
    const state = pageProps[APOLLO_STATE_PROP_NAME];
    return useMemo(() => initializeApollo(state), [state]);
}
