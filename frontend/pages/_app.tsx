import type {AppContext, AppInitialProps} from 'next/app'
import Head from 'next/head'
import '@fontsource/source-sans-pro'
import {ApolloProvider} from '@apollo/client'
import {useApollo} from '../lib/apollo'
import React from 'react'
import '@navikt/ds-css'
import {useUserInfoQuery} from "../lib/schema/graphql";
import PageLayout from "../components/pageLayout";


const MyApp = ({Component, pageProps}: AppInitialProps & AppContext) => {
    const apolloClient = useApollo(pageProps)
    return (
        <ApolloProvider client={apolloClient}>
            <Head>
                <link
                    rel='apple-touch-icon'
                    sizes='180x180'
                    href='/apple-touch-icon.png'
                />
                <link
                    rel='icon'
                    type='image/png'
                    sizes='32x32'
                    href='/favicon-32x32.png'
                />
                <link
                    rel='icon'
                    type='image/png'
                    sizes='16x16'
                    href='/favicon-16x16.png'
                />
                <link rel='manifest' href='/site.webmanifest'/>
                <link rel='mask-icon' href='/safari-pinned-tab.svg' color='#5bbad5'/>
                <meta name='msapplication-TileColor' content='#00aba9'/>
                <meta name='theme-color' content='#ffffff'/>
                <title>Fasit</title>
            </Head>
            <PageLayout>
                <Component {...pageProps} />
            </PageLayout>
        </ApolloProvider>
    )
}

export default MyApp