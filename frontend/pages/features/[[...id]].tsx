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
import { useFeatureDetailsQuery } from '../../lib/schema/graphql'

const Features = () => {
  const router = useRouter()
  const featureName = router.query.id as string

  const { data, error, loading } = useFeatureDetailsQuery({})

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
              </MenuItem>
            )
          })}
        </MenuItems>
      </SideMenu>
      <Main>
        {featureName && data && (
          <Feature feature={data.features.find((f) => f.name == featureName)} />
        )}
      </Main>
    </PageContainer>
  )
}
export default Features
