import gql from 'graphql-tag'

export const PARTNERS_CREATE = gql`
  mutation partnerCreate($name: String!, $description: String) {
    partnerCreate(partner: { name: $name, description: $description }) {
      id
    }
  }
`
