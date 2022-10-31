import { ErrorColored, SuccessColored } from '@navikt/ds-icons'
import { Loader, Accordion } from '@navikt/ds-react'
import styled from 'styled-components'
import {
  RolloutEventType,
  RolloutStatus,
  RolloutSummaryQuery,
} from '../../lib/schema/graphql'
import humanizeDate from '../lib/humanizeDate'

const typeText = (type: RolloutEventType) => {
  switch (type) {
    case RolloutEventType.Failed:
      return 'Failed rollout'
    case RolloutEventType.Success:
      return 'Successfully rolled out'
    case RolloutEventType.HelmCompleted:
      return 'Installed in CI environment'
    case RolloutEventType.Processed:
      return 'Rollout accepted'
    case RolloutEventType.InProgress:
      return 'Sent to CI environment'
    case RolloutEventType.RolledBack:
      return 'Rollout aborted - rolling back'
    default:
      return type
  }
}

type EventProps = {
  header?: boolean
}
export const Event = styled.div<EventProps>`
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

export const DateField = styled.span`
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

export const rolloutStatus = (status: RolloutStatus) => {
  const style = { marginRight: '10px' }

  switch (status) {
    case RolloutStatus.Deployed:
      return (
        <>
          <SuccessColored style={style} /> Sucessfully rolled out
        </>
      )
    case RolloutStatus.Failed:
      return (
        <>
          <ErrorColored style={style} /> Failed rolling out
        </>
      )
    default:
      return (
        <>
          <Loader style={style} />
          Rolling out{' '}
        </>
      )
  }
}

type RolloutProps = {
  rollout: RolloutSummaryQuery['rolloutSummary']['rollouts'][0]
}

const Rollout = ({ rollout }: RolloutProps) => {
  return (
    <Accordion.Item defaultOpen={rollout.status !== RolloutStatus.Deployed}>
      <Accordion.Header>
        {rolloutStatus(rollout.status)}
        Rollout to {rollout.environment.kind.toLowerCase()}
      </Accordion.Header>
      <Accordion.Content>
        {rollout.events.map((e, i) => {
          return (
            <div key={i}>
              <Event>
                <div>
                  {e.type === RolloutEventType.Failed ? (
                    <ErrorColored />
                  ) : (
                    <SuccessColored />
                  )}{' '}
                  {typeText(e.type)}
                </div>
                <DateField>{humanizeDate(e.created, 'PPPP', true)}</DateField>
              </Event>
              {Object.keys(e.data).length > 0 && (
                <Details>
                  {e.data.logs && e.data.logs}
                  {e.data.error && e.data.error}
                </Details>
              )}
            </div>
          )
        })}
      </Accordion.Content>
    </Accordion.Item>
  )
}

export default Rollout
