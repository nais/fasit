import gql from 'graphql-tag'

export const USERINFO = gql`
  query UserInfo {
    userInfo {
      email
    }
  }
`
