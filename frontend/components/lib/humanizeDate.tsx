import { format, parseISO } from 'date-fns'
import { nb } from 'date-fns/locale'
const humanizeDate = (isoDate: string, dateFormat = 'PPPP') => {
  try {
    const parsed = parseISO(isoDate);
    return <time dateTime={isoDate} title={format(parsed, "dd. MMMM yyyy HH:mm:ii", { locale: nb })}>
      {format(parsed, dateFormat, { locale: nb })}
    </time>
  } catch (e) {
    return <></>
  }
}
export default humanizeDate
