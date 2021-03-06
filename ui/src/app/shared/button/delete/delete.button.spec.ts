/* tslint:disable:no-unused-variable */

import {TestBed, tick, fakeAsync} from '@angular/core/testing';
import { TranslateService, TranslateLoader, TranslateParser} from 'ng2-translate';
import {DeleteButtonComponent} from '../../button/delete/delete.button';
import {SharedModule} from '../../shared.module';

describe('CDS: Delete Button', () => {

    beforeEach(() => {
        TestBed.configureTestingModule({
            declarations: [
            ],
            providers: [
                TranslateService,
                TranslateLoader,
                TranslateParser
            ],
            imports : [
                SharedModule
            ]
        });
    });

    it('Test emit delete event', fakeAsync( () => {
        // Create loginComponent
        let fixture = TestBed.createComponent(DeleteButtonComponent);
        let component = fixture.debugElement.componentInstance;
        expect(component).toBeTruthy();

        spyOn(fixture.componentInstance.event, 'emit');

        fixture.detectChanges();
        tick(50);

        let compiled = fixture.debugElement.nativeElement;
        expect(compiled.querySelector('.ui.red.button')).toBeTruthy('Delete button must be displayed');
        compiled.querySelector('.ui.red.button').click();

        fixture.detectChanges();
        tick(50);

        expect(compiled.querySelector('.ui.buttons')).toBeTruthy('Confirmation buttons must be displayed');
        compiled.querySelector('.ui.red.button.active').click();

        expect(fixture.componentInstance.event.emit).toHaveBeenCalledWith(true);
    }));
});

