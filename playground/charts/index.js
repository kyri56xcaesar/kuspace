function setupCanvas(canvas, width, height) {
    const dpr = window.devicePixelRatio || 1;
    canvas.width = width * dpr;
    canvas.height = height * dpr;
    canvas.style.width = width + 'px';
    canvas.style.height = height + 'px';

    const ctx = canvas.getContext('2d');

    return ctx;
}

function round(value, decimals) {
    return Number(Math.round(value + 'e' + decimals) + 'e-' + decimals);
  }

class myChart {
    constructor(canvas, ctx, how) {
        this.canvas = canvas;
        this.ctx = ctx;
        this.layers = [];
        this.currentAngle = -0.5;

        this.how = how;
        this.centerX = this.canvas.width / 2;
        this.centerY = this.canvas.height / 2;
        this.R = this.centerX * this.how.Rscale;
        this.offset = this.how.offset;
    }

    addLayer(layer) {
        this.layers.push(layer);
    }

    drawIndicators() {

        ctx.font = this.how.fontWidth + 'px Arial';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.fillStyle = 'black';


        // center 
        ctx.beginPath();
        ctx.arc(this.centerX, this.centerY, 5, 0, 2 * Math.PI)
        ctx.fill()

        // perimeter
        ctx.beginPath();
        ctx.lineWidth = 3;
        ctx.arc(this.centerX, this.centerY, this.R, 0, 2 * Math.PI);
        ctx.stroke();
        

        // rays/radius
        for (let i = 0; i <= this.how.max; i += this.how.step) {
            const angle = (i / this.how.max) * 2 * Math.PI - 0.5 * Math.PI;
                
            const xStart = this.centerX + Math.cos(angle) * (this.R - this.how.tick / 2);
            const yStart = this.centerY + Math.sin(angle) * (this.R - this.how.tick / 2);
    
            const xEnd = this.centerX + Math.cos(angle) * (this.R + this.how.tick / 2);
            const yEnd = this.centerY + Math.sin(angle) * (this.R + this.how.tick / 2);
            
    
            ctx.lineWidth = 3;
            ctx.beginPath();
            ctx.moveTo(xStart, yStart);
            ctx.lineTo(xEnd, yEnd);
            ctx.stroke();
            ctx.lineWidth = 1;

            ctx.beginPath();
            ctx.setLineDash(this.how.rays);
            ctx.moveTo(this.centerX, this.centerY);
            ctx.lineTo(xEnd, yEnd);
            ctx.stroke();

            ctx.setLineDash([]);

    
            // Optional: Draw value labels
            if (i == this.how.max) {
                continue;
            }
            const xText = this.centerX + Math.cos(angle) * (this.R + this.how.tick + this.offset);
            const yText = this.centerY + Math.sin(angle) * (this.R + this.how.tick + this.offset);
            
            ctx.fillText(i.toString(), xText, yText);
        }        
    }

    draw() {
        this.currentAngle = -0.5 * Math.PI; // start from the top
    
        this.layers.forEach(layer => {
            let rotation = layer.draw(this.currentAngle); // still returning radians
            this.currentAngle += rotation;
        });
    
        this.drawIndicators();
    }
    
    
}

class Layer {
    constructor(canvas, ctx, howMuch, style) {
        this.max = howMuch.max;
        this.target = howMuch.at;
        this.current = 0;

        this.ctx = ctx;

        this.name = style.name;
        this.color = style.color;
        this.padding = style.padding || 25;
        this.RScale = style.RScale || 25;

        this.where = {X: canvas.width / 2, Y: canvas.height / 2, R: (canvas.width / 2) * this.RScale - this.padding};
    }


    draw(startAngle) {
        // console.log('starting at (radians):', startAngle);
        
        const usageAngle = (this.target / this.max) * 2 * Math.PI;
        const endAngle = startAngle + usageAngle;
    
        // indicator darw
        const radius = this.where.R;
        const indicatorOffset = 0;
        const xEnd = this.where.X + (radius + indicatorOffset) * Math.cos(endAngle);
        const yEnd = this.where.Y + (radius + indicatorOffset) * Math.sin(endAngle);
    
        this.ctx.beginPath(); 
        this.ctx.moveTo(this.where.X, this.where.Y);
        this.ctx.strokeStyle = "black";
        this.ctx.lineWidth = 4;
        this.ctx.lineTo(xEnd, yEnd);
        this.ctx.stroke(); 
        this.ctx.lineWidth = 1;

    

        // label draw
        const midAngle = startAngle + usageAngle / 2;
        const labelRadius = radius * 1.25; // distance from center for label
        const labelX = this.where.X + labelRadius * Math.cos(midAngle);
        const labelY = this.where.Y + labelRadius * Math.sin(midAngle);
        
        this.ctx.fillStyle = "black";
        this.ctx.font = "14px sans-serif";
        this.ctx.textAlign = "center";
        this.ctx.textBaseline = "middle";
        this.ctx.fillStyle = this.color;
        this.ctx.fillText(this.name, labelX, labelY);

        // Draw the arc sector
        this.ctx.beginPath();
        this.ctx.fillStyle = this.color;
        this.ctx.moveTo(this.where.X, this.where.Y);
        this.ctx.arc(this.where.X, this.where.Y, radius, startAngle, endAngle);
        this.ctx.fill();
    
        return usageAngle;
    }
    

    
}


const canvas = document.getElementById('chart');
const ctx = setupCanvas(canvas, 600, 600); // logical dimensions (CSS size)

const padding = 1;
const RScale = 0.5;

const chart = new myChart(canvas, ctx, {max: 100, step: 5, tick: 10, offset: 3, Rscale: RScale, rays: [0, 10], fontWidth: 10});

const layer1 = new Layer(canvas, ctx, {max:100, at: 50}, {name:'Layer 1', color:'rgba(66, 164, 177, 0.5)', padding: padding, RScale: RScale});
const layer2 = new Layer(canvas, ctx, {max: 100, at: 4}, {name:'Layer 2', color:'rgba(25, 145, 109, 0.5)', padding: padding, RScale: RScale});
const layer3 = new Layer(canvas, ctx, {max: 100, at: 6}, {name:'Layer 3', color:'rgba(100, 197, 176, 0.5)', padding: padding, RScale: RScale});
const layer4 = new Layer(canvas, ctx, {max: 100, at: 30}, {name:'Layer 4', color:'rgba(4, 33, 67, 0.5)', padding: padding, RScale: RScale});
const layer5 = new Layer(canvas, ctx, {max: 100, at: 10}, {name:'Layer 5', color:'rgba(156, 18, 115, 0.5)', padding: padding, RScale: RScale});

// console.log(layer1)
chart.addLayer(layer1);
chart.addLayer(layer2);
chart.addLayer(layer3);
chart.addLayer(layer4);
chart.addLayer(layer5);

chart.draw();
