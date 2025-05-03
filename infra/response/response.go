package response

import "encoding/json"

type Response struct {
	Code    int            `json:"code"`
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

func (r *Response) SetCode(code int) *Response {
	r.Code = code
	return r
}

func (r *Response) SetSuccess(success bool) *Response {
	r.Success = success
	return r
}

func (r *Response) SetMessage(message string) *Response {
	r.Message = message
	return r
}

func (r *Response) SetData(key string, value any) *Response {
	r.Data[key] = value
	return r
}

func (r *Response) SetModel(model any, key string) error {
	data, err := json.Marshal(r.Data[key])
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &model)
	if err != nil {
		return err
	}
	return nil
}
