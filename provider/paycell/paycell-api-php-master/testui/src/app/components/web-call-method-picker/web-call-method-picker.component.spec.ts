import { async, ComponentFixture, TestBed } from '@angular/core/testing';

import { WebCallMethodPickerComponent } from './web-call-method-picker.component';

describe('WebCallMethodPickerComponent', () => {
  let component: WebCallMethodPickerComponent;
  let fixture: ComponentFixture<WebCallMethodPickerComponent>;

  beforeEach(async(() => {
    TestBed.configureTestingModule({
      declarations: [ WebCallMethodPickerComponent ]
    })
    .compileComponents();
  }));

  beforeEach(() => {
    fixture = TestBed.createComponent(WebCallMethodPickerComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
