/** @type {import('next').NextConfig} */

// for react-md-editor. see https://github.com/uiwjs/react-md-editor#support-nextjs
const removeImports = require('next-remove-imports')()

const withBundleAnalyzer = require('@next/bundle-analyzer')({
  enabled: process.env.ANALYZE === 'true',
})

module.exports = withBundleAnalyzer(removeImports({
  reactStrictMode: true,
  compiler: {
    // Enables the styled-components SWC transform
    styledComponents: true
  }
}))