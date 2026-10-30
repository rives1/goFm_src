#!/usr/bin/env python3
# Draws winres/icon32.png and winres/icon48.png pixel by pixel (v1.13): at these sizes a
# scaled-down icon.png is blurred. Needs Pillow; run from the winres folder.
from PIL import Image, ImageDraw
BG=(10,25,70,255); Y=(255,225,77,255); G=(90,110,154,255)
L=[(213,222,238,255),(42,179,200,255),(121,192,255,255),(90,247,142,255)]
R=[(213,222,238,255),(255,121,198,255),(199,146,234,255),(78,230,200,255)]
def bg(S, y0, y1, rad):
    k=8; big=Image.new('RGBA',(S*k,S*k),(0,0,0,0))
    ImageDraw.Draw(big).rounded_rectangle([0,y0*k,S*k-1,(y1+1)*k-1],radius=rad*k,fill=BG)
    return big.resize((S,S),Image.LANCZOS)
def make(S,y0,y1,rad,panels,stroke,barx,bary,barh):
    im=bg(S,y0,y1,rad); px=im.load()
    def rect(x0,y0_,x1,y1_,c):
        for y in range(y0_,y1_+1):
            for x in range(x0,x1+1): px[x,y]=c
    for (x0,py0,x1,py1),col,bars in zip(panels,(Y,G),(L,R)):
        for s in range(stroke):
            rect(x0+s,py0+s,x1-s,py0+s,col); rect(x0+s,py1-s,x1-s,py1-s,col)
            rect(x0+s,py0+s,x0+s,py1-s,col); rect(x1-s,py0+s,x1-s,py1-s,col)
        if stroke>=3:  # soften the outer corners
            for cx,cy in ((x0,py0),(x1,py0),(x0,py1),(x1,py1)): px[cx,cy]=BG
        for i,by in enumerate(bary):
            bx0=x0+barx; bx1=x1-barx
            rect(bx0,by,bx1,by+barh-1,bars[i])
            if barh>=3:
                for cx,cy in ((bx0,by),(bx1,by),(bx0,by+barh-1),(bx1,by+barh-1)): px[cx,cy]=BG
    return im
make(32,2,29,4,[(3,5,14,26),(17,5,28,26)],2,3,[9,13,17,21],2).save('icon32.png',optimize=True)
make(48,3,44,6,[(4,7,21,40),(26,7,43,40)],3,5,[13,19,25,31],3).save('icon48.png',optimize=True)
