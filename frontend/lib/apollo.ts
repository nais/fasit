import {useMemo} from 'react'
import {ApolloClient, HttpLink, InMemoryCache, NormalizedCacheObject, split} from '@apollo/client'
import merge from 'deepmerge'
import isEqual from 'lodash.isequal'
import {GraphQLWsLink} from "@apollo/client/link/subscriptions";
import {createClient} from "graphql-ws";
import {getMainDefinition} from "@apollo/client/utilities";
import WebSocket from 'ws'

const isServer = typeof window === 'undefined'

export const APOLLO_STATE_PROP_NAME = '__APOLLO_STATE__'

let apolloClient: ApolloClient<NormalizedCacheObject> | undefined

const getQueryURI = () => {
    if (process.env.NEXT_PUBLIC_ENV === 'development') {
        return '://localhost:8080/query'
    }
    return isServer ? `://fasit-backend/query` : `s://${window.location.host}/query`
}

const httpLink = (cookie: string | undefined) => {
    return new HttpLink({
        uri: `http${getQueryURI()}`,
        credentials: 'include',
        headers: {
            cookie
        }
    })
};

const wsLink = new GraphQLWsLink(createClient({
    webSocketImpl: isServer ? WebSocket : undefined,
    url: `ws${getQueryURI()}`,
}));

// Not using wsLink at the moment, but kept for future use.
const splitLink = (cookie: string | undefined) => {
    return split(
        ({query}) => {
            const definition = getMainDefinition(query);
            return (
                definition.kind === 'OperationDefinition' &&
                definition.operation === 'subscription'
            );
        },
        wsLink,
        httpLink(cookie),)
}

const createApolloClient = (cookie?: string) => {
    return new ApolloClient({
        ssrMode: typeof window === 'undefined',
        link: splitLink(cookie),
        cache: new InMemoryCache({
                typePolicies: {
                    Configuration: {
                        keyFields: [["key"], ["id"]],
                    },
                    GlobalConfiguration: {
                        keyFields: [["key"], ["id"]],
                    }

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


export const useApollo = (pageProps: any) => {
    const state = pageProps[APOLLO_STATE_PROP_NAME];
    return useMemo(() => initializeApollo(state), [state]);
}
