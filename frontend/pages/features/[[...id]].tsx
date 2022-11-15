import { Warning } from '@navikt/ds-icons'
import { useRouter } from 'next/router'
import Feature from '../../components/features/feature'
import ErrorMessage from '../../components/lib/error'
import {
  Main,
  MenuItem,
  MenuItems,
  PageContainer,
  SideMenu,
} from '../../components/lib/PageLayout'
import LoaderSpinner from '../../components/lib/spinner'
import { useFeatureListQuery } from '../../lib/schema/graphql'
import { navOransje } from '../../styles/constants'

const Features = () => {
  const router = useRouter()
  const featureNameParam = router.query.id
  let featureName = featureNameParam as string
  if (Array.isArray(featureNameParam)) {
    featureName = featureNameParam[0]
  }

  const { data, error, loading } = useFeatureListQuery({})

  if (error) {
    return <ErrorMessage error={error} />
  }

  return (
    <PageContainer>
      <SideMenu width={200}>
        {loading || (!data && <LoaderSpinner />)}
        {error && <ErrorMessage error={error} />}
        <MenuItems>
          {data?.features?.map((e, i) => {
            return (
              <MenuItem
                onClick={() => router.push(`/features/${e.name}`)}
                key={`${e.name}_${i}`}
                active={e.name == featureName}
              >
                <a>{e.name}</a>
                {e.outdatedInfo.length > 0 && (
                  <>
                    {' '}
                    <Warning style={{ color: navOransje }} />
                  </>
                )}
              </MenuItem>
            )
          })}
        </MenuItems>
      </SideMenu>
      <Main>
        {featureName && data && <Feature featureName={featureName} />}
      </Main>
    </PageContainer>
  )
}
export default Features
