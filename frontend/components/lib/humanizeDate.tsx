import { format, parseISO, formatDistance } from 'date-fns'
import { enGB } from 'date-fns/locale'
const humanizeDate = (
  isoDate: string,
  dateFormat = 'PPPP',
  distance: boolean = false,
) => {
  try {
    const parsed = parseISO(isoDate)
    return (
      <time
        dateTime={isoDate}
        title={format(parsed, 'dd. MMMM yyyy HH:mm:ss', { locale: enGB })}
      >
        {distance
          ? formatDistance(parsed, Date.now(), {
              addSuffix: true,
              includeSeconds: true,
            })
          : format(parsed, dateFormat, { locale: enGB })}
      </time>
    )
  } catch (e) {
    return <></>
  }
}
export default humanizeDate
