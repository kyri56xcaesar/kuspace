function setupCanvas(canvas, width, height) {
    const dpr = window.devicePixelRatio || 1;
    canvas.width = width * dpr;
    canvas.height = height * dpr;
    canvas.style.width = width + 'px';
    canvas.style.height = height + 'px';

    const ctx = canvas.getContext('2d');

    return ctx;
}


class myChart {
    constructor(canvas, ctx, how) {
        this.canvas = canvas;
        this.ctx = ctx;
        this.layers = [];

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

        this.ctx.font = this.how.fontWidth + 'px Arial';
        this.ctx.textAlign = 'center';
        this.ctx.textBaseline = 'middle';
        this.ctx.fillStyle = 'black';

        this.ctx.beginPath();
        this.ctx.arc(this.centerX, this.centerY, this.R, 0, 2 * Math.PI);
        this.ctx.stroke();
        
        for (let i = 0; i <= this.how.max; i += this.how.step) {
            const angle = (i / this.how.max) * 2 * Math.PI - 0.5 * Math.PI;
                
            const xStart = this.centerX + Math.cos(angle) * (this.R - this.how.tick / 2);
            const yStart = this.centerY + Math.sin(angle) * (this.R - this.how.tick / 2);
    
            const xEnd = this.centerX + Math.cos(angle) * (this.R + this.how.tick / 2);
            const yEnd = this.centerY + Math.sin(angle) * (this.R + this.how.tick / 2);
            
    
            this.ctx.lineWidth = 3;
            this.ctx.beginPath();
            this.ctx.moveTo(xStart, yStart);
            this.ctx.lineTo(xEnd, yEnd);
            this.ctx.stroke();
            this.ctx.lineWidth = 1;

            this.ctx.beginPath();
            this.ctx.setLineDash(this.how.rays);
            this.ctx.moveTo(this.centerX, this.centerY);
            this.ctx.lineTo(xEnd, yEnd);
            this.ctx.stroke();

            this.ctx.setLineDash([]);

    
            // Optional: Draw value labels
            if (i == this.how.max) {
                continue;
            }
            const xText = this.centerX + Math.cos(angle) * (this.R + this.how.tick + this.offset);
            const yText = this.centerY + Math.sin(angle) * (this.R + this.how.tick + this.offset);
            
            this.ctx.fillText(i.toString(), xText, yText);
        }        
    }

    draw() {
        this.layers.forEach(layer => {
            layer.draw();
        });
        this.drawIndicators();
        
    }

    animate() {
        const needsRedraw = this.layers.some(layer => layer.update());
    
        this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
        this.draw(); // calls layer.draw() + indicators
    
        if (needsRedraw) {
            requestAnimationFrame(() => this.animate());
        }
    }
    

}

class Layer {
    constructor(chart, howMuch, style) {
        this.max = howMuch.max;
        this.target = howMuch.at;
        this.current = 0;

        this.ctx = chart.ctx;

        this.name = style.name;
        this.color = style.color;
        this.padding = style.padding || 25;

        this.where = {X: chart.canvas.width / 2, Y: chart.canvas.height / 2, R: (chart.canvas.width / 2) - this.padding};
    }


    draw() {
        const usageAngle = (this.target / this.max) * 2 * Math.PI;
        this.ctx.beginPath();
        this.ctx.fillStyle = this.color;
        this.ctx.moveTo(this.where.X, this.where.Y);
        this.ctx.arc(this.where.X, this.where.Y, this.where.R, -0.5 * Math.PI, usageAngle - 0.5 * Math.PI);
        this.ctx.fill();
    }

    update() {
        const speed = 0.1; // percent per frame — you can tweak this
        if (this.current < this.target) {
            this.current += speed;
            if (this.current > this.target) this.current = this.target;
            return true; // still animating
        }
        return false; // done
    }

    
}


// const canvas = document.getElementById('chart');
// const ctx = setupCanvas(canvas, 250, 250); // logical dimensions (CSS size)

// const chart = new myChart(canvas, ctx, {max: 100, step: 5, tick: 10, offset: 0, Rscale: 0.9, rays: [5, 5], fontWidth: 15});
// const padding = 15;

// const layer1 = new Layer(canvas, {max:100, at: 46}, {name:'Layer 1', color:'rgba(66, 164, 177, 0.5)', padding: padding});
// const layer2 = new Layer(canvas, {max: 100, at: 67}, {name:'Layer 2', color:'rgba(25, 145, 109, 0.5)', padding: padding});
// const layer3 = new Layer(canvas, {max: 100, at: 97}, {name:'Layer 3', color:'rgba(100, 197, 176, 0.5)', padding: padding});

// // console.log(layer1)
// chart.addLayer(layer1);
// chart.addLayer(layer2);
// chart.addLayer(layer3);

// chart.animate();
