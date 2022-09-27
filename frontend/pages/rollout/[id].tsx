import { useRouter } from "next/router"
import { RolloutEventType, RolloutStatus, useRolloutQuery } from "../../lib/schema/graphql"
import LoaderSpinner from "../../components/lib/spinner"
import ErrorMessage from "../../components/lib/error"
import styled from "styled-components"
import humanizeDate from "../../components/lib/humanizeDate"
import { ErrorColored, SuccessColored } from "@navikt/ds-icons"
import { Loader } from "@navikt/ds-react"

const DateField = styled.span`
  font-size: 0.8rem;
  color: #666;
  font-weight: normal;
`

const Details = styled.div`
  margin-left: 30px;
  border: 1px solid silver;
  border-radius: 5px;
  padding: 15px 10px;
  background-color: #f5f5f5;
  overflow: auto;
  word-break: break-word;
  white-space: pre-wrap;
  font-size: 12px;
`
type EventProps = {
  header?: boolean
}
const Event = styled.div<EventProps>`
 display: flex; 
 align-items: center; 
 justify-content: space-between; 
 margin: 10px 0;
 ${(props) => props.header && `font-weight: bold;`}
 ${(props) => props.header && `font-size: 1.5em;`}
 > div { 
     display: flex;
     align-items: center;
     gap: 10px;
 }
`

const typeText = (type: RolloutEventType) => {
  switch (type) {
    case RolloutEventType.Failed:
      return "Failed rollout"
    case RolloutEventType.Success:
      return "Successfully rolled out"
    case RolloutEventType.HelmCompleted:
      return "Installed in CI environment"
    case RolloutEventType.Processed:
      return "Rollout accepted"
    case RolloutEventType.InProgress:
      return "Sent to CI environment"
    case RolloutEventType.RolledBack:
      return "Rollout aborted - rolling back"
    default:
      return type
  }
}

const rolloutStatus = (status: RolloutStatus) => {

  const style = { marginRight: "10px" }

  switch (status) {
    case RolloutStatus.Deployed:
      return <><SuccessColored style={style} /> Sucessfully rolled out</>
    case RolloutStatus.Failed:
      return <><ErrorColored style={style} /> Failed rolling out</>
    default:
      return <><Loader style={style} />Rolling out </>
  }

}

const Rollout = () => {
  const id = useRouter().query.id as string
  const { data, loading, error, startPolling, stopPolling } = useRolloutQuery({ variables: { id } })

  if (error) return <ErrorMessage error={error} />
  if (!data || loading) return <LoaderSpinner />
  let lastEventTime = data.rollout.created
  if (data.rollout.events.length > 0) {
    lastEventTime = data.rollout.events[data.rollout.events.length - 1].created
  }
  if (data.rollout.status === RolloutStatus.Deployed || data.rollout.status === RolloutStatus.Failed) {
    stopPolling()
  } else {
    startPolling(2000)
  }
  return <>
    <Event header>
      <div style={{flexGrow: 1}}>
        {rolloutStatus(data.rollout.status)}
        <i>{data.rollout.feature.name}</i>
      </div>
      <DateField>{humanizeDate(data.rollout.created, "dd. MMMM yyyy - HH:mm:ii")}</DateField>
    </Event>
    {data.rollout.events.map((e, i) => {
      return <div key={i}>
        <Event>
          <div>
            {e.type === "FAILED" ? <ErrorColored /> : <SuccessColored />} {typeText(e.type)}
          </div>
          <DateField>{humanizeDate(e.created, "PPPP", true)}</DateField>
        </Event>
        {Object.keys(e.data).length > 0 && <Details>
          {e.data.logs && e.data.logs}
          {e.data.error && e.data.error}
        </Details>}

      </div>
    })}
    {data.rollout.status === "UNKNOWN" || data.rollout.status === "PENDING" &&
      <Event>
        <div>
          <Loader
            variant="neutral"
            size="medium"
            title="waiting"
          />
          Waiting for rollout to complete
        </div>
        <DateField>{humanizeDate(lastEventTime, "", true)}</DateField>
      </Event>
    }


  </>

}
export default Rollout