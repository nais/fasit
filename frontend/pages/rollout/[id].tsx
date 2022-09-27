import { useRouter } from "next/router"
import { useRolloutQuery } from "../../lib/schema/graphql"
import LoaderSpinner from "../../components/lib/spinner"
import ErrorMessage from "../../components/lib/error"
import styled from "styled-components"
import humanizeDate from "../../components/lib/humanizeDate"

const RolloutSummary = styled.div`
  border: 1px solid silver;
  border-radius: 5px;
  padding: 10px;
  background-color: #f5f5f5;
  font-size: 0.8em;
  margin-bottom: 10px;
`

const Rollout = () => {
  const id = useRouter().query.id as string
  const { data, loading, error } = useRolloutQuery({ variables: { id }, pollInterval: 2000 })
  if (error) return <ErrorMessage error={error} />
  if (!data || loading) return <LoaderSpinner />
  return <>
    <h1>Rollout - {data.rollout.feature.name}</h1>
    <RolloutSummary>
      <div>Rollout status: {data.rollout.status}</div>
      <div>Rollout created: {humanizeDate(data.rollout.created)}</div>
    </RolloutSummary>
    {data.rollout.events.map((e, i) => {
      return <div key={i}>
        type:{e.type}<br/>
        created: {humanizeDate(e.created)}<br/>
        {e.data.log && <div>message: {e.data.log}</div>}
        {e.data.error && <div>message: {e.data.error}</div>}
      </div>
    })}




  </>

}
export default Rollout