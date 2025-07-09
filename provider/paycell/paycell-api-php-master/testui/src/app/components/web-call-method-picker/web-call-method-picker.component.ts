import { Component, OnInit, Input } from '@angular/core';
import { State } from '../State';

@Component({
  selector: 'app-web-call-method-picker',
  templateUrl: './web-call-method-picker.component.html',
  styleUrls: ['./web-call-method-picker.component.css']
})
export class WebCallMethodPickerComponent implements OnInit {

  @Input() state: State;

  constructor() { }

  ngOnInit() {
  }

}
