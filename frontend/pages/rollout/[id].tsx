import { Accordion, Loader } from '@navikt/ds-react'
import { useRouter } from 'next/router'
import ErrorMessage from '../../components/lib/error'
import humanizeDate from '../../components/lib/humanizeDate'
import LoaderSpinner from '../../components/lib/spinner'
import Rollout, {
  DateField,
  Event,
  rolloutStatus,
} from '../../components/rollout/rollout'
import { RolloutStatus, useRolloutSummaryQuery } from '../../lib/schema/graphql'

const RolloutSummary = () => {
  const id = useRouter().query.id as string
  const { data, loading, error, startPolling, stopPolling } =
    useRolloutSummaryQuery({ variables: { id } })

  if (error) return <ErrorMessage error={error} />
  if (!data || loading) return <LoaderSpinner />
  let lastEventTime = data.rolloutSummary.created
  if (data.rolloutSummary.rollouts.flatMap((v) => v.events).length > 0) {
    lastEventTime = data.rolloutSummary.rollouts
      .flatMap((v) => v.events)
      .sort((a, b) => a.created - b.created)[0].created
  }
  if (
    data.rolloutSummary.status === RolloutStatus.Deployed ||
    data.rolloutSummary.status === RolloutStatus.Failed
  ) {
    stopPolling()
  } else {
    startPolling(2000)
  }
  return (
    <>
      <Event header>
        <div style={{ flexGrow: 1 }}>
          {rolloutStatus(data.rolloutSummary.status)}
          <i>{data.rolloutSummary.feature.name}</i>
        </div>
        <DateField>
          {humanizeDate(
            data.rolloutSummary.created,
            'dd. MMMM yyyy - HH:mm:ss',
          )}
        </DateField>
      </Event>
      <Accordion>
        {data.rolloutSummary.rollouts.map((rollout) => (
          <Rollout key={rollout.id} rollout={rollout} />
        ))}
      </Accordion>
      {data.rolloutSummary.status === 'UNKNOWN' ||
        (data.rolloutSummary.status === 'PENDING' && (
          <Event>
            <div>
              <Loader variant="neutral" size="medium" title="waiting" />
              Waiting for rollout to complete
            </div>
            <DateField>{humanizeDate(lastEventTime, '', true)}</DateField>
          </Event>
        ))}
    </>
  )
}
export default RolloutSummary
