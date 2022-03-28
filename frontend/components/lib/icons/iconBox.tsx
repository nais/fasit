import styled from 'styled-components'

export interface IconBoxProps {
  size: number
  height?: number
  children?: React.ReactNode
  justifyRight?: boolean
}

export const StyledIconBox = styled.div`
  display: flex;
  align-items: center;
  justify-content: center;
  margin-left: ${(p: IconBoxProps) => (p.justifyRight ? 'auto' : 'default')};
  margin: 0 auto;
  overflow: hidden;
  height: ${(p: IconBoxProps) => p.height? p.height :p.size}px;
  width: ${(p: IconBoxProps) => p.size}px;
`

export const IconBox = ({ size, height, children, justifyRight }: IconBoxProps) => (
  <StyledIconBox size={size} height={height} justifyRight={justifyRight}>
    {children}
  </StyledIconBox>
)

export default IconBox
