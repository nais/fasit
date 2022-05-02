import { format, parseISO } from 'date-fns'
import { enGB } from 'date-fns/locale'
const humanizeDate = (isoDate: string, dateFormat = 'PPPP') => {
  try {
    const parsed = parseISO(isoDate);
    return <time dateTime={isoDate} title={format(parsed, "dd. MMMM yyyy HH:mm:ii", { locale: enGB })}>
      {format(parsed, dateFormat, { locale: enGB })}
    </time>
  } catch (e) {
    return <></>
  }
}
export default humanizeDate
